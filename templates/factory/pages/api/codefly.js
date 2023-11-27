
const tempEnvConfig = {
    "CODEFLY-ENDPOINT__IAM__PEOPLE____REST":
        "http://localhost:11408",
    "CODEFLY-ENDPOINT__API__GATEWAY____REST": "http://localhost:11485",
};

export default function codefly({ endpoint, get }) {
    endpoint = endpoint.replace("/", "__");
    const envString = `CODEFLY-ENDPOINT__${endpoint}____REST`.toUpperCase();
    return tempEnvConfig[envString] + get;
}