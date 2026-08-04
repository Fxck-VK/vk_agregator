import styles from "./SocialCta.module.css";

export type SocialLink = { href: string; label: string };

export function SocialCta({ links = [] }: { links?: SocialLink[] }) {
  return (
    <div className={styles.social}>
      <p>Сообщество NeiroHub</p>
      <h2>Следите за развитием платформы</h2>
      {links.length > 0 ? (
        <div className={styles.links}>
          {links.map((link) => <a href={link.href} key={link.href} rel="noreferrer noopener" target="_blank">{link.label}</a>)}
        </div>
      ) : <span>Ссылки на официальные каналы появятся после их подтверждения.</span>}
    </div>
  );
}
