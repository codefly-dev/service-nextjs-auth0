import { useUser } from "@auth0/nextjs-auth0/client";
import Layout from "../components/layout";
import { UserInfoCard } from "../components/user-info-card";

const Home = () => {
  const { user, isLoading } = useUser();

  return (
    <Layout user={user} loading={isLoading}>
      {isLoading && <p>Loading login info...</p>}

      {!isLoading && !user && (
        <>
          <p>
            To test the login click on <i>Login</i>
          </p>
          <p>
            Once you have logged in you should be able to navigate between
            protected routes: client rendered, server rendered profile pages,
            and <i>Logout</i>
          </p>
        </>
      )}

      {user && (
        <>
          <UserInfoCard user={user} />
        </>
      )}
    </Layout>
  );
};

// fast/cached SSR page
export default Home;
