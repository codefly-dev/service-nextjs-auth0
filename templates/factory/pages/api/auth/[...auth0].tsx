import { handleAuth, handleLogin } from "@auth0/nextjs-auth0";
export default handleAuth({
  //   login: handleLogin({
  //     authorizationParams: {
  //       audience: "http://localhost:3000", // or AUTH0_AUDIENCE
  //       // Add the `offline_access` scope to also get a Refresh Token
  //       scope: "openid profile email", // or AUTH0_SCOPE
  //     },
  //   }),
});
