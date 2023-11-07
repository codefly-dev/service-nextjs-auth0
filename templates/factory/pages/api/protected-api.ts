import {getAccessToken, withApiAuthRequired} from '@auth0/nextjs-auth0'
import axios from "axios";

async function handle(req, res) {
    const {accessToken} = await getAccessToken(req, res)
    try {
        const r = await axios.get('http://localhost:8080/version', {headers: {Authorization: `BearerJU ${accessToken}`}})
        const data = r?.data
        console.log("data", data)
        res.status(200).json({
            session: 'true',
            version: data['version']
        })
    } catch (e) {
        res.status(500).json({error: 'Unable to fetch', description: e})
    }
}

export default withApiAuthRequired(handle)
