const DEFAULT_HOST = "local.drizzle.studio";
const DEFAULT_PORT = "4983";
const DEFAULT_PROTOCOL = "https:";

export type DrizzleStudioTarget = {
  host: string;
  port: string;
  protocol: string;
  href: string;
};

export function resolveStudioTarget(location: Location): DrizzleStudioTarget {
  const params = new URLSearchParams(location.search);
  const host = params.get("host")?.trim() || DEFAULT_HOST;
  const port = params.get("port")?.trim() || DEFAULT_PORT;
  const protocol = params.get("protocol")?.trim() || DEFAULT_PROTOCOL;
  const url = new URL(`${protocol}//${host}/`);
  url.searchParams.set("port", port);
  if (host !== DEFAULT_HOST) {
    url.searchParams.set("host", host);
  }
  return {
    host,
    port,
    protocol,
    href: url.toString(),
  };
}
