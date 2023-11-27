import { useUser } from "@auth0/nextjs-auth0/client";
import { FetchAndDisplayJSON } from "../components/fetch-and-display-json";
import Layout from "../components/layout";
import codefly from "./api/codefly";


const Home = () => {
  const { user, isLoading } = useUser();

  return (
    <Layout user={user} loading={isLoading}>
      {isLoading && <p>Loading login info...</p>}

      <div className="grid gap-[30px]">
        <div className="grid gap-1">
          <FetchAndDisplayJSON/>
        </div>
      </div>
    </Layout>
  );
};



export default Home;
