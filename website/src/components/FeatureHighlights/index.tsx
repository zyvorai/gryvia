import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
  to: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'GPU scheduling',
    description: 'Queues, priorities, gang scheduling and topology-aware placement for training and inference jobs.',
    to: '/docs/guides/SCHEDULING',
  },
  {
    title: 'Quotas & cost',
    description: 'Per-team GPU quotas, budgets and cost tracking, enforced by a Kubernetes operator.',
    to: '/docs/guides/STORAGE_NETWORK_OPERATORS',
  },
  {
    title: 'Network intelligence',
    description: 'eBPF flow telemetry and GPU-communication visibility, exported to Prometheus and Grafana.',
    to: '/docs/guides/NETWORK_INTELLIGENCE',
  },
  {
    title: 'ML workflows',
    description: 'Multi-step pipelines, model registry, checkpoint guards and inference services as custom resources.',
    to: '/docs/guides/ML_WORKFLOWS',
  },
  {
    title: 'CLI, SDKs & dashboard',
    description: 'A Rust CLI, Go and Python SDKs, and a web dashboard over the same API.',
    to: '/docs/guides/CLI_GUIDE',
  },
  {
    title: 'Kubernetes-native',
    description: 'Everything is a CRD and an operator. Install with Helm; no separate control plane.',
    to: '/docs/getting-started/quickstart',
  },
];

function Feature({title, description, to}: FeatureItem) {
  return (
    <div className="col col--4">
      <Link to={to} className={styles.card}>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </Link>
    </div>
  );
}

export default function FeatureHighlights(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
