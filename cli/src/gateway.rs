//! A small client for the Gryvia API gateway (`GRYVIA_GATEWAY_URL`), used by the commands whose data lives
//! behind it (collector counters, merged network flows and graph, security alerts).
//!
//! Authentication is the gateway's API key sent as a bearer token (`GRYVIA_API_KEY`), the same way the
//! docs use `curl -H "Authorization: Bearer $GRYVIA_API_KEY"`. TLS is verified against the system roots or
//! `GRYVIA_CA_FILE`; `--insecure` (or `GRYVIA_INSECURE=1`) turns verification off and says so on stderr.
//! Only GET is implemented: the CLI never changes anything through the gateway.
//!
//! Verified against an in-process mock gateway (plain HTTP, and TLS with a test CA); not run against a real
//! gateway deployment.

use std::sync::Arc;
use std::time::Duration;

use anyhow::{anyhow, bail, Context, Result};
use http_body_util::{BodyExt, Empty, Limited};
use hyper::body::Bytes;
use hyper::{Method, Request, Uri};
use hyper_rustls::HttpsConnectorBuilder;
use hyper_util::client::legacy::Client;
use hyper_util::rt::TokioExecutor;
use rustls::client::danger::{HandshakeSignatureValid, ServerCertVerified, ServerCertVerifier};
use rustls::pki_types::pem::PemObject;
use rustls::pki_types::{CertificateDer, ServerName, UnixTime};
use rustls::{ClientConfig, DigitallySignedStruct, RootCertStore, SignatureScheme};

/// Response bodies larger than this are refused.
const MAX_BODY: usize = 16 * 1024 * 1024;
const DEFAULT_TIMEOUT: Duration = Duration::from_secs(15);

/// Where and how to reach the gateway.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct GatewayConfig {
    /// Base URL without a trailing slash, for example `https://gryvia.example.com`.
    pub base_url: String,
    pub api_key: Option<String>,
    pub insecure: bool,
    pub ca_file: Option<String>,
    pub timeout: Duration,
}

impl GatewayConfig {
    /// The gateway from `--gateway` / `GRYVIA_GATEWAY_URL`, or `None` when neither is set (kube-only mode).
    pub fn resolve(flag_url: Option<&str>, insecure_flag: bool) -> Option<Self> {
        Self::resolve_with(flag_url, insecure_flag, |k| std::env::var(k).ok())
    }

    pub fn resolve_with(
        flag_url: Option<&str>,
        insecure_flag: bool,
        env: impl Fn(&str) -> Option<String>,
    ) -> Option<Self> {
        let clean = |s: String| {
            let t = s.trim().trim_end_matches('/').to_string();
            (!t.is_empty()).then_some(t)
        };
        let base_url = flag_url
            .map(|s| s.to_string())
            .or_else(|| env("GRYVIA_GATEWAY_URL"))
            .and_then(clean)?;
        let timeout = env("GRYVIA_GATEWAY_TIMEOUT")
            .and_then(|s| s.trim().parse::<u64>().ok())
            .filter(|s| *s > 0)
            .map(Duration::from_secs)
            .unwrap_or(DEFAULT_TIMEOUT);
        Some(Self {
            base_url,
            api_key: env("GRYVIA_API_KEY")
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty()),
            insecure: insecure_flag || env("GRYVIA_INSECURE").as_deref() == Some("1"),
            ca_file: env("GRYVIA_CA_FILE")
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty()),
            timeout,
        })
    }
}

/// A GET-only gateway client.
#[derive(Clone, Debug)]
pub struct GatewayClient {
    cfg: GatewayConfig,
}

impl GatewayClient {
    pub fn new(cfg: GatewayConfig) -> Self {
        Self { cfg }
    }

    /// GET `path` (starting with `/`, may carry a query) and parse the JSON body.
    pub async fn get_json(&self, path: &str) -> Result<serde_json::Value> {
        let url = format!("{}{}", self.cfg.base_url, path);
        let uri: Uri = url
            .parse()
            .with_context(|| format!("invalid gateway URL {url}"))?;
        let mut req = Request::builder()
            .method(Method::GET)
            .uri(uri)
            .header("accept", "application/json")
            .header(
                "user-agent",
                concat!("gryvia-cli/", env!("CARGO_PKG_VERSION")),
            );
        if let Some(key) = &self.cfg.api_key {
            req = req.header("authorization", format!("Bearer {key}"));
        }
        let req = req.body(Empty::<Bytes>::new())?;

        let connector = HttpsConnectorBuilder::new()
            .with_tls_config(self.tls_config()?)
            .https_or_http()
            .enable_http1()
            .build();
        let client: Client<_, Empty<Bytes>> =
            Client::builder(TokioExecutor::new()).build(connector);

        let fut = async {
            let resp = client
                .request(req)
                .await
                .map_err(|e| anyhow!("{}", error_chain(&e)))?;
            let status = resp.status();
            let body = Limited::new(resp.into_body(), MAX_BODY)
                .collect()
                .await
                .map_err(|e| anyhow!("reading the gateway response: {e}"))?
                .to_bytes();
            Ok::<_, anyhow::Error>((status, body))
        };
        let (status, body) = tokio::time::timeout(self.cfg.timeout, fut)
            .await
            .map_err(|_| {
                anyhow!(
                    "gateway {} did not answer within {}s",
                    self.cfg.base_url,
                    self.cfg.timeout.as_secs()
                )
            })?
            .map_err(|e| anyhow!("gateway {} is not reachable: {e}", self.cfg.base_url))?;

        if !status.is_success() {
            let detail = String::from_utf8_lossy(&body);
            bail!(
                "{}",
                error_text(&self.cfg.base_url, path, status.as_u16(), &detail)
            );
        }
        serde_json::from_slice(&body)
            .with_context(|| format!("gateway {path} returned a body that is not JSON"))
    }

    fn tls_config(&self) -> Result<ClientConfig> {
        let provider = Arc::new(rustls::crypto::ring::default_provider());
        let builder = ClientConfig::builder_with_provider(provider.clone())
            .with_safe_default_protocol_versions()
            .context("TLS protocol setup")?;
        if self.cfg.insecure {
            eprintln!("warning: --insecure: the gateway certificate is NOT verified");
            return Ok(builder
                .dangerous()
                .with_custom_certificate_verifier(Arc::new(NoVerify(
                    provider
                        .signature_verification_algorithms
                        .supported_schemes(),
                )))
                .with_no_client_auth());
        }
        let mut roots = RootCertStore::empty();
        if let Some(path) = &self.cfg.ca_file {
            let mut n = 0;
            for cert in CertificateDer::pem_file_iter(path)
                .with_context(|| format!("cannot read GRYVIA_CA_FILE {path}"))?
            {
                roots.add(cert.context("GRYVIA_CA_FILE holds an invalid certificate")?)?;
                n += 1;
            }
            if n == 0 {
                bail!("GRYVIA_CA_FILE {path} holds no certificate");
            }
        } else {
            let loaded = rustls_native_certs::load_native_certs();
            for cert in loaded.certs {
                let _ = roots.add(cert);
            }
        }
        Ok(builder.with_root_certificates(roots).with_no_client_auth())
    }
}

/// An error and its causes on one line (`client error (Connect): ... invalid peer certificate: ...`).
fn error_chain(e: &dyn std::error::Error) -> String {
    let mut out = e.to_string();
    let mut cur = e.source();
    while let Some(c) = cur {
        let t = c.to_string();
        if !out.contains(&t) {
            out.push_str(": ");
            out.push_str(&t);
        }
        cur = c.source();
    }
    out
}

/// Concise, actionable text for a non-2xx gateway answer.
pub fn error_text(base: &str, path: &str, status: u16, body: &str) -> String {
    let detail = serde_json::from_str::<serde_json::Value>(body)
        .ok()
        .and_then(|v| {
            v.get("detail")
                .and_then(|d| d.as_str())
                .map(|s| s.to_string())
        })
        .unwrap_or_else(|| {
            body.lines()
                .next()
                .unwrap_or("")
                .chars()
                .take(120)
                .collect()
        });
    let hint = match status {
        401 => " (set GRYVIA_API_KEY to a valid API key)",
        403 => " (this key is not allowed to read it; the network routes need an admin key)",
        404 => " (the gateway does not serve this route; is it an older version?)",
        429 => " (rate limited; retry in a minute)",
        _ => "",
    };
    let detail = if detail.is_empty() {
        String::new()
    } else {
        format!(": {detail}")
    };
    format!("gateway {base}{path} answered HTTP {status}{detail}{hint}")
}

#[derive(Debug)]
struct NoVerify(Vec<SignatureScheme>);

impl ServerCertVerifier for NoVerify {
    fn verify_server_cert(
        &self,
        _end_entity: &CertificateDer<'_>,
        _intermediates: &[CertificateDer<'_>],
        _server_name: &ServerName<'_>,
        _ocsp: &[u8],
        _now: UnixTime,
    ) -> Result<ServerCertVerified, rustls::Error> {
        Ok(ServerCertVerified::assertion())
    }
    fn verify_tls12_signature(
        &self,
        _m: &[u8],
        _c: &CertificateDer<'_>,
        _d: &DigitallySignedStruct,
    ) -> Result<HandshakeSignatureValid, rustls::Error> {
        Ok(HandshakeSignatureValid::assertion())
    }
    fn verify_tls13_signature(
        &self,
        _m: &[u8],
        _c: &CertificateDer<'_>,
        _d: &DigitallySignedStruct,
    ) -> Result<HandshakeSignatureValid, rustls::Error> {
        Ok(HandshakeSignatureValid::assertion())
    }
    fn supported_verify_schemes(&self) -> Vec<SignatureScheme> {
        self.0.clone()
    }
}

/// A tiny in-process HTTP/1.1 server for tests: maps a request path (without query) to (status, body).
/// It records the `Authorization` header of every request.
#[cfg(test)]
pub mod mock {
    use std::collections::HashMap;
    use std::sync::{Arc, Mutex};

    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener;

    pub struct MockGateway {
        pub url: String,
        pub auth_seen: Arc<Mutex<Vec<Option<String>>>>,
    }

    pub async fn start(routes: Vec<(&'static str, u16, String)>) -> MockGateway {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let url = format!("http://{}", listener.local_addr().unwrap());
        let table: Arc<HashMap<String, (u16, String)>> = Arc::new(
            routes
                .into_iter()
                .map(|(p, s, b)| (p.to_string(), (s, b)))
                .collect(),
        );
        let auth_seen = Arc::new(Mutex::new(Vec::new()));
        let a2 = auth_seen.clone();
        tokio::spawn(async move {
            loop {
                let Ok((mut sock, _)) = listener.accept().await else {
                    return;
                };
                let (table, a2) = (table.clone(), a2.clone());
                tokio::spawn(async move {
                    let mut buf = vec![0u8; 16384];
                    let n = sock.read(&mut buf).await.unwrap_or(0);
                    let text = String::from_utf8_lossy(&buf[..n]).to_string();
                    let first = text.lines().next().unwrap_or("");
                    let full = first.split_whitespace().nth(1).unwrap_or("/").to_string();
                    let path = full.split('?').next().unwrap_or("/").to_string();
                    let auth = text.lines().find_map(|l| {
                        l.to_ascii_lowercase()
                            .starts_with("authorization:")
                            .then(|| l[14..].trim().to_string())
                    });
                    a2.lock().unwrap().push(auth);
                    let (status, body) = table
                        .get(&path)
                        .cloned()
                        .unwrap_or((404, "{\"detail\":\"Not Found\"}".to_string()));
                    let resp = format!(
                        "HTTP/1.1 {status} X\r\ncontent-type: application/json\r\ncontent-length: {}\r\nconnection: close\r\n\r\n{body}",
                        body.len()
                    );
                    let _ = sock.write_all(resp.as_bytes()).await;
                    let _ = sock.shutdown().await;
                });
            }
        });
        MockGateway { url, auth_seen }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn env_of<'a>(pairs: &'a [(&'a str, &'a str)]) -> impl Fn(&str) -> Option<String> + 'a {
        move |k| {
            pairs
                .iter()
                .find(|(n, _)| *n == k)
                .map(|(_, v)| v.to_string())
        }
    }

    #[test]
    fn config_resolution() {
        assert_eq!(GatewayConfig::resolve_with(None, false, env_of(&[])), None);
        assert_eq!(
            GatewayConfig::resolve_with(None, false, env_of(&[("GRYVIA_GATEWAY_URL", "  ")])),
            None
        );
        let c = GatewayConfig::resolve_with(
            None,
            false,
            env_of(&[
                ("GRYVIA_GATEWAY_URL", "https://gw.example.com/"),
                ("GRYVIA_API_KEY", " k "),
                ("GRYVIA_GATEWAY_TIMEOUT", "3"),
            ]),
        )
        .unwrap();
        assert_eq!(c.base_url, "https://gw.example.com");
        assert_eq!(c.api_key.as_deref(), Some("k"));
        assert_eq!(c.timeout, Duration::from_secs(3));
        assert!(!c.insecure);
        // the flag wins over the environment; --insecure or GRYVIA_INSECURE=1 turn verification off
        let c = GatewayConfig::resolve_with(
            Some("http://flag:1"),
            true,
            env_of(&[("GRYVIA_GATEWAY_URL", "http://env:2")]),
        )
        .unwrap();
        assert_eq!(c.base_url, "http://flag:1");
        assert!(c.insecure);
        let c = GatewayConfig::resolve_with(
            Some("http://x"),
            false,
            env_of(&[("GRYVIA_INSECURE", "1")]),
        )
        .unwrap();
        assert!(c.insecure);
    }

    #[test]
    fn error_texts_are_concise() {
        let t = error_text("http://gw", "/api/x", 401, "{\"detail\":\"Invalid token\"}");
        assert!(
            t.contains("HTTP 401") && t.contains("Invalid token") && t.contains("GRYVIA_API_KEY")
        );
        assert!(error_text("http://gw", "/api/x", 404, "nope").contains("does not serve"));
        assert!(!error_text("http://gw", "/api/x", 500, "a\nb").contains('\n'));
    }

    #[tokio::test]
    async fn get_json_sends_bearer_and_parses() {
        let gw = mock::start(vec![("/api/network/graph", 200, "{\"nodes\":[]}".into())]).await;
        let client = GatewayClient::new(GatewayConfig {
            base_url: gw.url.clone(),
            api_key: Some("secret-key".into()),
            insecure: false,
            ca_file: None,
            timeout: Duration::from_secs(5),
        });
        let v = client.get_json("/api/network/graph").await.unwrap();
        assert_eq!(v["nodes"].as_array().unwrap().len(), 0);
        assert_eq!(
            gw.auth_seen.lock().unwrap()[0].as_deref(),
            Some("Bearer secret-key")
        );
    }

    #[tokio::test]
    async fn http_errors_and_unreachable_are_reported() {
        let gw = mock::start(vec![(
            "/api/x",
            403,
            "{\"detail\":\"Admin access required\"}".into(),
        )])
        .await;
        let mk = |url: String| {
            GatewayClient::new(GatewayConfig {
                base_url: url,
                api_key: None,
                insecure: false,
                ca_file: None,
                timeout: Duration::from_secs(2),
            })
        };
        let err = mk(gw.url.clone()).get_json("/api/x").await.unwrap_err();
        assert!(err.to_string().contains("HTTP 403"), "{err}");
        let err = mk("http://127.0.0.1:1".into())
            .get_json("/api/x")
            .await
            .unwrap_err();
        assert!(err.to_string().contains("not reachable"), "{err}");
    }

    // TLS: a server with a self-signed test certificate (src/testdata, valid for 127.0.0.1 and localhost).
    async fn tls_server() -> String {
        use std::sync::Arc;
        use tokio::io::{AsyncReadExt, AsyncWriteExt};
        use tokio_rustls::TlsAcceptor;

        let certs: Vec<CertificateDer<'static>> =
            CertificateDer::pem_slice_iter(include_bytes!("testdata/gw-test.crt"))
                .collect::<Result<_, _>>()
                .unwrap();
        let key = rustls::pki_types::PrivateKeyDer::from_pem_slice(include_bytes!(
            "testdata/gw-test.key"
        ))
        .unwrap();
        let cfg = rustls::ServerConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()
        .unwrap()
        .with_no_client_auth()
        .with_single_cert(certs, key)
        .unwrap();
        let acceptor = TlsAcceptor::from(Arc::new(cfg));
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let url = format!("https://{}", listener.local_addr().unwrap());
        tokio::spawn(async move {
            loop {
                let Ok((sock, _)) = listener.accept().await else {
                    return;
                };
                let acceptor = acceptor.clone();
                tokio::spawn(async move {
                    let Ok(mut tls) = acceptor.accept(sock).await else {
                        return; // a client that rejects the certificate aborts the handshake
                    };
                    let mut buf = vec![0u8; 4096];
                    let _ = tls.read(&mut buf).await;
                    let body = "{\"ok\":true}";
                    let resp = format!(
                        "HTTP/1.1 200 OK\r\ncontent-type: application/json\r\ncontent-length: {}\r\nconnection: close\r\n\r\n{body}",
                        body.len()
                    );
                    let _ = tls.write_all(resp.as_bytes()).await;
                    let _ = tls.shutdown().await;
                });
            }
        });
        url
    }

    fn tls_client(url: String, insecure: bool, ca: Option<&str>) -> GatewayClient {
        GatewayClient::new(GatewayConfig {
            base_url: url,
            api_key: None,
            insecure,
            ca_file: ca.map(|s| s.to_string()),
            timeout: Duration::from_secs(5),
        })
    }

    #[tokio::test]
    async fn tls_is_verified_against_ca_file_and_rejected_otherwise() {
        let url = tls_server().await;
        let ca = concat!(env!("CARGO_MANIFEST_DIR"), "/src/testdata/gw-test-ca.crt");
        // pinned CA: verifies (the certificate carries the IP as a SAN)
        let v = tls_client(url.clone(), false, Some(ca))
            .get_json("/x")
            .await
            .unwrap();
        assert_eq!(v["ok"], true);
        // system roots do not know the test CA: refused
        let err = tls_client(url.clone(), false, None)
            .get_json("/x")
            .await
            .unwrap_err();
        assert!(err.to_string().contains("not reachable"), "{err}");
        // --insecure skips verification
        let v = tls_client(url, true, None).get_json("/x").await.unwrap();
        assert_eq!(v["ok"], true);
        // a CA file that does not exist is an error, not a silent fallback
        let err = tls_client(
            "https://127.0.0.1:1".into(),
            false,
            Some("/nonexistent/ca.pem"),
        )
        .get_json("/x")
        .await
        .unwrap_err();
        assert!(err.to_string().contains("GRYVIA_CA_FILE"), "{err}");
    }
}
