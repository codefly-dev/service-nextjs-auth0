import { useUser } from "@auth0/nextjs-auth0/client";
import { Endpoints } from "../components/endpoints";
import { FetchAndDisplayJSON } from "../components/fetch-and-display-json";
import Layout from "../components/layout";
import { useState } from "react";
import { JSONView } from "../components/json-view";
import useSWR from "swr";

export const callApi = async (url, token) => {
  try {
    const response = await fetch(url, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    return response.json();
  } catch (error) {
    console.error('Error calling the API', error);
    throw error;
  }
};

const Home = ({
                endpoints,
                restRoutes,
                restEndpoints,
              }: {
  endpoints: Record<string, string>;
  restRoutes: Record<string, string>;
  restEndpoints: Record<string, string>;
}) => {
  const { user, isLoading } = useUser();
  const [selectedEndpoint, setSelectedEndpoint] = useState("");

  const restPaths = {};
  Object.keys(restRoutes).map((key) => {
    const [endpoint, prop] = key
        .replace("CODEFLY_RESTROUTE__", "")
        .replace("___REST", "")
        .split("____", 2);
    restPaths[endpoint] = prop.replaceAll("__", "/").toLowerCase();
  });


  const { data: accessToken } = useSWR("/api/access-token", (url) =>
      fetch(url).then((res) => res.json())
  );

  const {
    data,
    error,
    isLoading: isLoadingEndpoint,
  } = useSWR(
      selectedEndpoint
          ? `http://${restEndpoints[selectedEndpoint]}/${restPaths[selectedEndpoint]}`
          : null,
      (url) => callApi(url, accessToken?.data)
  );

  return (
      <Layout user={user} loading={isLoading}>
        {isLoading && <p>Loading login info...</p>}

        <div className="grid gap-[30px]">
          {/* <div className="grid gap-1">
          <FetchAndDisplayJSON endpoints={endpoints} />
        </div>

        <div>
          <h4 className="mb-1">Endpoints</h4>
          <Endpoints endpoints={endpoints} />
        </div> */}

          <div className="w-full">
            <h4 className="mb-1">Endpoints & REST route</h4>
            <select
                value={selectedEndpoint}
                onChange={({ target }) => setSelectedEndpoint(target.value)}
                className="w-full p-[10px] rounded-[5px] border-[1px] border-[#ddd]"
            >
              <option value="">Choose an endpoint</option>
              {Object.keys(restEndpoints).map((key) => (
                  <option key={key} value={key}>
                    {key.replace("__", "/").toLowerCase()} ({restPaths[key]})
                  </option>
              ))}
            </select>

            {!!selectedEndpoint && (
                <>
                  <p className="mt-2">
                    In the code, we fetch the data with the SDK
                  </p>
                  <JSONView>{`codefly({ endpoint: "${selectedEndpoint
                      .replace("__", "/")
                      .toLowerCase()}", get: "/${
                      restPaths[selectedEndpoint]
                  }" })`}</JSONView>

                  <div className="mt-4">
                    <h4 className="mb-1">Data loaded</h4>
                    <JSONView>
                      {isLoadingEndpoint
                          ? "Loading..."
                          : data
                              ? JSON.stringify(data, null, 2)
                              : error
                                  ? "Error loading data"
                                  : "Choose an endpoint"}
                    </JSONView>
                  </div>
                </>
            )}
          </div>
        </div>
      </Layout>
  );
};

export async function getServerSideProps() {
  const endpoints = {};
  Object.keys(process.env).forEach((key) => {
    if (key.startsWith("CODEFLY_ENDPOINT__")) {
      endpoints[key] = process.env[key];
    }
  });

  const restEndpoints = {};
  Object.keys(process.env).forEach((key) => {
    if (key.startsWith("CODEFLY_ENDPOINT__") && key.includes("___REST")) {
      restEndpoints[
          key.replace("CODEFLY_ENDPOINT__", "").replace("___REST", "")
          ] = process.env[key];
    }
  });

  const restRoutes = {};
  Object.keys(process.env).forEach((key) => {
    if (key.startsWith("CODEFLY_RESTROUTE__")) {
      restRoutes[key] = process.env[key];
    }
  });

  return { props: { endpoints, restRoutes, restEndpoints } };
}

export default Home;
