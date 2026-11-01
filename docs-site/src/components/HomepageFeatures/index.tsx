import type { ReactNode } from 'react';
import styles from './styles.module.css';
import Link from '@docusaurus/Link'; // 推荐使用 Docusaurus 的 Link 组件以获得更好的 SPA 体验

type FeatureItem = {
  title: string;
  icon: ReactNode;
  description: ReactNode;
  link?: string;
  linkLabel?: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: '极速部署，开箱即用',
    icon: (
      // Icon: Rocket / Box
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/>
        <path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/>
        <path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/>
        <path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/>
      </svg>
    ),
    description: (
      <>
        原生支持 <strong>Docker</strong> 与二进制直连，无缝集成 <strong>Cloudflare Tunnel</strong>，无需复杂配置即可实现安全的内网穿透与外网访问。
      </>
    ),
    link: '/docs/Deployment',
    linkLabel: '部署指南'
  },
  {
    title: '智能检测，自动调优',
    icon: (
      // Icon: Gauge / Activity
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="m12 14 4-4"/>
        <path d="M3.34 19a10 10 0 1 1 17.32 0"/>
      </svg>
    ),
    description: (
      <>
        内置高性能并发测速与流媒体解锁检测。全自动化的<strong>内存管理</strong>与<strong>版本更新</strong>机制，确保服务长期稳定、零维护运行。
      </>
    ),
    link: '/docs/Features-Details',
    linkLabel: '功能详解'
  },
  {
    title: '灵活订阅，多端同步',
    icon: (
      // Icon: Workflow / Share
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="18" cy="5" r="3"/>
        <circle cx="6" cy="12" r="3"/>
        <circle cx="18" cy="19" r="3"/>
        <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
        <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
      </svg>
    ),
    description: (
      <>
        自适应生成 <strong>Mihomo</strong>、<strong>Sing-box</strong> 等主流格式配置。集成 Sub-Store 逻辑，支持私有分享码、文件代理与即时通知。
      </>
    ),
    link: '/docs/Subscriptions',
    linkLabel: '订阅文档'
  },
];

function Feature({ title, icon, description, link, linkLabel }: FeatureItem) {
  return (
    <div className={styles.card}>
      <div className={styles.cardHeader}>
        <div className={styles.iconWrapper}>{icon}</div>
        <h3 className={styles.cardTitle}>{title}</h3>
      </div>
      <p className={styles.cardDesc}>{description}</p>
      {link && (
        <Link className={styles.cardLink} to={link}>
          {linkLabel} <span className={styles.arrow}>→</span>
        </Link>
      )}
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className={styles.grid}>
        {FeatureList.map((props, idx) => (
          <Feature key={idx} {...props} />
        ))}
      </div>
    </section>
  );
}