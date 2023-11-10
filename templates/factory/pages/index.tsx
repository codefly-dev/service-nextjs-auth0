import { useUser } from "@auth0/nextjs-auth0/client";
import { FetchAndDisplayJSON } from "../components/fetch-and-display-json";
import Layout from "../components/layout";

const tempEnvConfig = {
  "CODEFLY-ENDPOINT__IAM__PEOPLE___VERSION____REST":
    "https://dummyjson.com/products/1",
};

const Home = () => {
  const { user, isLoading } = useUser();

  return (
    <Layout user={user} loading={isLoading}>
      {isLoading && <p>Loading login info...</p>}

      <div className="grid gap-[30px]">
        <div className="grid gap-1">
          <h4 className="font-semibold">Protected Route</h4>
          <FetchAndDisplayJSON
            url={codefly({ endpoint: "iam/people", get: "/version" })}
            protectedRoute
          />
        </div>

        <div className="grid gap-1">
          <h4 className="font-semibold">Public Route</h4>
          <FetchAndDisplayJSON
            url={codefly({ endpoint: "iam/people", get: "/version" })}
          />
        </div>
      </div>
    </Layout>
  );
};

function codefly({ endpoint, get }: { endpoint: string; get: string }) {
  endpoint = endpoint.replace("/", "__");
  const envString = `CODEFLY-ENDPOINT__${endpoint}___${get.replace(
    "/",
    ""
  )}____REST`.toUpperCase();
  return tempEnvConfig[envString];
}

export default Home;
