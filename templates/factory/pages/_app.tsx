import { UserProvider } from "@auth0/nextjs-auth0/client";
import "../styles/globals.css";

export default function App({ Component, pageProps }) {
  const { user } = pageProps;

  return (
    <UserProvider user={user}>
      <Component {...pageProps} />
    </UserProvider>
  );
}
