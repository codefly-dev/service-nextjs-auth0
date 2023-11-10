import useSWR from "swr";
import { JSONView } from "./json-view";

export function FetchAndDisplayJSON({
  url,
  protectedRoute,
}: {
  url: string;
  protectedRoute?: boolean;
}) {
  console.log(url);
  const { data: accessToken } = useSWR("/api/access-token", (url) =>
    fetch(url).then((res) => res.json())
  );

  const { data, error, isLoading } = useSWR(
    !protectedRoute
      ? [url]
      : protectedRoute && accessToken
      ? [url, accessToken]
      : null,
    ([url, accessToken]) =>
      fetch(url, {
        headers: { Authorization: `Bearer ${accessToken}` },
      }).then((res) => res.json())
  );

  return isLoading ? (
    <pre>Loading...</pre>
  ) : error || !data ? (
    <pre>Error loading data</pre>
  ) : (
    <JSONView>{JSON.stringify(data)}</JSONView>
  );
}
