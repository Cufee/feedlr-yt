import { getFetchClient as getUpstreamFetchClient } from "getFetchClient";

// Some Google media servers close the connection after HEAD. Reusing that
// connection for Companion's POST can fail before any media is returned.
const client = Deno.createHttpClient({ poolMaxIdlePerHost: 0 });

export function getFetchClient(config) {
  const upstreamFetch = getUpstreamFetchClient(config);
  if (config.networking.proxy || config.networking.ipv6_block) {
    return upstreamFetch;
  }
  return (input, init) => upstreamFetch(input, { client, ...init });
}
