import { useUser } from "@auth0/nextjs-auth0/client";
import { Endpoints } from "../components/endpoints";
import { FetchAndDisplayJSON } from "../components/fetch-and-display-json";
import Layout from "../components/layout";

const Home = ({ endpoints }) => {
  const { user, isLoading } = useUser();

  return (
    <Layout user={user} loading={isLoading}>
      {isLoading && <p>Loading login info...</p>}

      <div className="grid gap-[30px]">
        <div className="grid gap-1">
          <FetchAndDisplayJSON endpoints={endpoints} />
        </div>

        <div>
          <h4 className="mb-1">Endpoints</h4>
          <Endpoints endpoints={endpoints} />
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
  return { props: { endpoints } };
}

export default Home;
