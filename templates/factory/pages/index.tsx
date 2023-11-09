import { useUser } from "@auth0/nextjs-auth0/client";
import Layout from "../components/layout";
import { UserInfoCard } from "../components/user-info-card";

const Home = () => {
  const { user, isLoading } = useUser();

  return (
    <Layout user={user} loading={isLoading}>
      {isLoading && <p>Loading login info...</p>}
    </Layout>
  );
};

// fast/cached SSR page
export default Home;
