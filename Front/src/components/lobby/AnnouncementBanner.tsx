import type { LobbyAnnouncement } from './types';
import { SparkIcon } from './icons';

type Props = {
  announcement: LobbyAnnouncement;
  onAction: () => void;
};

/**
 * AnnouncementBanner met en avant une actualité, sans bloquer les actions.
 * Le repère vert signale que le village est actif maintenant.
 */
export default function AnnouncementBanner({ announcement, onAction }: Props) {
  return (
    <aside className="lobby-announcement" aria-labelledby={`announcement-${announcement.id}`}>
      <span className="lobby-announcement-icon" aria-hidden="true">
        <SparkIcon />
      </span>
      <div className="lobby-announcement-copy">
        <p className="lobby-announcement-live">
          <span className="live-dot" aria-hidden="true" />
          En direct
        </p>
        <p className="lobby-announcement-title" id={`announcement-${announcement.id}`}>
          {announcement.title}
        </p>
        <p className="lobby-announcement-text">{announcement.text}</p>
      </div>
      <button type="button" className="btn small" onClick={onAction}>
        {announcement.actionLabel}
      </button>
    </aside>
  );
}
