import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import FeatureHighlights from '@site/src/components/FeatureHighlights';
import Reveal from '@site/src/components/Reveal';

import styles from './index.module.css';

function HomepageHeader() {
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container text--center">
        <Heading as="h1" className="hero__title">
          Open GPU orchestration
          <br />
          for Kubernetes.
        </Heading>
        <p className="hero__subtitle">
          Schedule, share and observe GPU workloads with operators and CRDs you already know how to run.
        </p>
        <div className={clsx(styles.buttons, styles.buttonsCentered)}>
          <Link className="button button--secondary button--lg" to="/docs/getting-started/quickstart">
            Get started
          </Link>
          <Link
            className="button button--outline button--lg button--secondary"
            to="https://github.com/zyvorai/gryvia">
            View on GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

function ProjectStatus() {
  return (
    <section className={styles.problem}>
      <div className="container">
        <Reveal className="row">
          <div className="col col--8 col--offset-2 text--center">
            <Heading as="h2" className={styles.sectionHeading}>
              Early, and honest about it
            </Heading>
            <p>
              Gryvia is in active development. The API is <code>gryvia.io/v1</code>, and it may change
              before a stable release. Performance figures are targets until they are backed by
              published benchmark results.
            </p>
            <Link to="/docs/guides/ROADMAP">Read the roadmap →</Link>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Gryvia — open GPU orchestration for Kubernetes"
      description="Open GPU orchestration for Kubernetes: scheduling, quotas, network intelligence and ML workflows as operators and CRDs.">
      <HomepageHeader />
      <main>
        <Reveal>
          <FeatureHighlights />
        </Reveal>
        <ProjectStatus />
      </main>
    </Layout>
  );
}
