import axios from "axios";

export default async function PublicRoute(req, res) {
    try {
        const r = await axios.get('http://localhost:11736/version')
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