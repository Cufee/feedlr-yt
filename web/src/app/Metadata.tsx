import { useRouterState } from "@tanstack/react-router";

const defaultDescription = "A thoughtful home for the YouTube channels and podcasts you choose.";

export function Metadata({
  title,
  description = defaultDescription,
  image = "/assets/og.jpg",
  author,
  durationSeconds,
  kind = "website",
}: {
  title: string;
  description?: string;
  image?: string;
  author?: string;
  durationSeconds?: number | null;
  kind?: string;
}) {
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const canonical = new URL(pathname, window.location.origin).href;
  const artwork = new URL(image, window.location.origin).href;
  const pageTitle = `${title} · Feedlr`;
  return (
    <>
      <title>{pageTitle}</title>
      <meta name="description" content={description} />
      <link rel="canonical" href={canonical} />
      <meta property="og:url" content={canonical} />
      <meta property="og:title" content={pageTitle} />
      <meta property="og:description" content={description} />
      <meta property="og:type" content={kind} />
      <meta property="og:image" content={artwork} />
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:title" content={pageTitle} />
      <meta name="twitter:description" content={description} />
      <meta name="twitter:image" content={artwork} />
      <meta name="twitter:image:alt" content={title} />
      {author && <meta name="author" content={author} />}
      {!!durationSeconds && (
        <>
          <meta property="video:duration" content={String(durationSeconds)} />
          <meta property="og:video:duration" content={String(durationSeconds)} />
          <meta itemProp="duration" content={`PT${Math.trunc(durationSeconds)}S`} />
        </>
      )}
    </>
  );
}
