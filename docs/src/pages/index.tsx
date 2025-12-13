import React from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <img src="img/ghost.svg" alt="Spectre Logo" width="200" />
        <h1 className="hero__title">{siteConfig.title}</h1>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Get Started
          </Link>
          <Link
            className="button button--outline button--secondary button--lg"
            to="https://github.com/moolen/spectre"
            style={{marginLeft: '1rem'}}>
            View on GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

function Feature({title, description, emoji}) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center" style={{fontSize: '3rem', marginBottom: '1rem'}}>
        {emoji}
      </div>
      <div className="text--center padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          <Feature
            emoji="🔍"
            title="Real-time Event Capture"
            description="Capture every Kubernetes resource change (create, update, delete) across your cluster in real-time using Kubernetes watch API."
          />
          <Feature
            emoji="⚡"
            title="Efficient Storage"
            description="Block-based compressed storage with 90%+ compression ratio. Bloom filters and sparse indexing enable fast queries even on large datasets."
          />
          <Feature
            emoji="📊"
            title="Interactive Timeline"
            description="Visualize resource state changes over time with an intuitive React-based UI. Filter by namespace, kind, or time range."
          />
        </div>
        <div className="row" style={{marginTop: '2rem'}}>
          <Feature
            emoji="🤖"
            title="AI-Assisted Analysis"
            description="Model Context Protocol (MCP) integration enables Claude and other AI assistants to help with incident investigation and post-mortem analysis."
          />
          <Feature
            emoji="🎯"
            title="Incident Investigation"
            description="Debug issues and understand the sequence of events that led to problems. Perfect for incident response and troubleshooting."
          />
          <Feature
            emoji="🚀"
            title="Easy Deployment"
            description="Deploy with Helm, Docker, or as a binary. Supports any Kubernetes cluster with minimal resource requirements."
          />
        </div>
      </div>
    </section>
  );
}

function HomepageScreenshot() {
  return (
    <section className={styles.screenshot}>
      <div className="container">
        <div className="row">
          <div className="col">
            <h2 className="text--center" style={{marginBottom: '2rem'}}>
              Visualize Resource Changes Over Time
            </h2>
            <div className="text--center">
              <img
                src="img/screenshot-2.png"
                alt="Spectre Timeline Screenshot"
                style={{maxWidth: '100%', border: '1px solid var(--ifm-color-emphasis-300)', borderRadius: '8px'}}
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title} - ${siteConfig.tagline}`}
      description="Kubernetes event monitoring and auditing system for incident investigation and troubleshooting">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <HomepageScreenshot />
        <section style={{padding: '4rem 0', textAlign: 'center'}}>
          <div className="container">
            <h2>Ready to get started?</h2>
            <p style={{fontSize: '1.2rem', marginBottom: '2rem'}}>
              Install Spectre in your Kubernetes cluster in minutes
            </p>
            <Link
              className="button button--primary button--lg"
              to="/docs/getting-started/quick-start">
              Quick Start Guide →
            </Link>
          </div>
        </section>
      </main>
    </Layout>
  );
}
