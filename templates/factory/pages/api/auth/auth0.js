import { initAuth0 } from '@auth0/nextjs-auth0';

export default initAuth0({
    secret: process.env.AUTH0_SECRET,
    baseURL: process.env.AUTH0_BASE_URL,
    issuerBaseURL: process.env.AUTH0_ISSUER_BASE_URL,
    clientID: process.env.AUTH0_CLIENT_ID,
    clientSecret: process.env.AUTH0_CLIENT_SECRET,

    // Optional configurations
    routes: {
        callback: '/api/auth/callback',
        postLogoutRedirect: '/'
    },
    session: {
        cookieSecret: process.env.SESSION_COOKIE_SECRET,
        cookieLifetime: 60 * 60 * 8,
        storeIdToken: false,
        storeAccessToken: false,
        storeRefreshToken: false
    }
});