/**
 * 对话消息（SPEC-V5 §8.1）：用户与 Vit 只靠底色区分（二选一，不用徽章）。
 * 正文 16/400/1.65；行宽约 36 个汉字。
 */
export interface MessageProps {
  author: "user" | "vit";
  children: React.ReactNode;
  className?: string;
}

export function Message({ author, children, className }: MessageProps) {
  const cls = ["v5-message", `v5-message--${author}`, className]
    .filter(Boolean)
    .join(" ");
  return (
    <p className={cls} data-message-author={author}>
      {children}
    </p>
  );
}
