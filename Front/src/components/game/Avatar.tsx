import { avatarEmoji, avatarStyle } from './avatars';

type Props = {
  /** Place du joueur dans la table : l'avatar reste stable tant qu'il est là. */
  index: number;
  dead?: boolean;
};

/** Avatar est la pastille d'un joueur : un emoji et une teinte tirés de sa place. */
export default function Avatar({ index, dead }: Props) {
  return (
    <span className={dead ? 'avatar gone' : 'avatar'} style={avatarStyle(index)} aria-hidden="true">
      {avatarEmoji(index)}
    </span>
  );
}
