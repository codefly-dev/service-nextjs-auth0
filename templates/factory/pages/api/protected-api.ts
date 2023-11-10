import { getAccessToken, withApiAuthRequired } from "@auth0/nextjs-auth0";

const API_URL = process.env["CODEFLY-ENDPOINT__APP__SVC__COOL__REST"];

async function handle(req, res) {
  const { accessToken } = await getAccessToken(req, res);
  try {
    const response = await fetch(API_URL, {
      headers: { Authorization: `Bearer ${accessToken}` },
    });
    const data = await response.json();
    res.status(200).json(data);
  } catch (e) {
    res.status(500).json({ error: "Unable to fetch", description: e });
  }
}

export default withApiAuthRequired(handle);
