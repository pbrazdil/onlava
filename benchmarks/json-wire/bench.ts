import Client from "./.generated/client.ts"

const baseURL = process.env.BASE_URL ?? "http://127.0.0.1:48191"
const iterations = Number(process.env.ITERATIONS ?? "2500")
const concurrency = Number(process.env.CONCURRENCY ?? "24")
const itemCount = Number(process.env.ITEMS ?? "24")
const acceptEncoding = process.env.ACCEPT_ENCODING

function makePayload(items: number) {
    const meta: Record<string, string> = {}
    for (let i = 0; i < 12; i++) {
        meta[`key_${i}`] = `value-${i}-${"x".repeat(12)}`
    }
    return {
        name: "scenery-wire-benchmark",
        count: 7,
        items: Array.from({ length: items }, (_, i) => ({
            id: `item-${i}`,
            count: i + 1,
            active: i % 2 === 0,
            score: i * 1.25,
            tags: [`tag-${i % 5}`, `bucket-${i % 3}`, "scenery"],
        })),
        meta,
    }
}

function percentile(sorted: number[], q: number) {
    const index = Math.min(sorted.length - 1, Math.max(0, Math.ceil(sorted.length * q) - 1))
    return sorted[index]
}

async function byteLength(body: BodyInit | null | undefined) {
    if (body === undefined || body === null) return 0
    if (typeof body === "string") return new TextEncoder().encode(body).byteLength
    if (body instanceof Uint8Array) return body.byteLength
    if (body instanceof ArrayBuffer) return body.byteLength
    if (body instanceof Blob) return body.size
    return -1
}

async function measureBytes(label: string, client: Client, payload: ReturnType<typeof makePayload>) {
    const calls: unknown[] = []
    const measured = client.with({
        fetcher: async (input, init) => {
            const response = await fetch(input, init)
            calls.push({
                url: String(input).replace(baseURL, ""),
                method: init?.method ?? "GET",
                request_bytes: await byteLength(init?.body),
                response_content_length: response.headers.get("content-length"),
                response_encoding: response.headers.get("content-encoding") ?? "identity",
                response_decoded_bytes: (await response.clone().arrayBuffer()).byteLength,
                content_type: response.headers.get("content-type"),
            })
            return response
        },
    })
    await measured.bench.Echo(payload)
    return { label, calls }
}

async function bench(label: string, client: Client, payload: ReturnType<typeof makePayload>) {
    const expectedTotal = payload.count + payload.items.reduce((sum, item) => sum + item.count, 0)
    const samples = new Array<number>(iterations)
    let next = 0

    const started = performance.now()
    await Promise.all(Array.from({ length: concurrency }, async () => {
        while (true) {
            const index = next++
            if (index >= iterations) {
                return
            }
            const before = performance.now()
            const response = await client.bench.Echo(payload)
            const elapsed = performance.now() - before
            if (response.total !== expectedTotal || response.item_count !== payload.items.length) {
                throw new Error(`${label}: invalid response ${JSON.stringify(response)}`)
            }
            samples[index] = elapsed
        }
    }))
    const wallMs = performance.now() - started
    const sorted = samples.slice().sort((a, b) => a - b)
    const sum = samples.reduce((acc, value) => acc + value, 0)
    return {
        label,
        wall_ms: wallMs,
        req_per_sec: iterations / (wallMs / 1000),
        avg_ms: sum / samples.length,
        p50_ms: percentile(sorted, 0.50),
        p95_ms: percentile(sorted, 0.95),
        p99_ms: percentile(sorted, 0.99),
    }
}

function formatRow(result: Awaited<ReturnType<typeof bench>>) {
    return [
        result.label.padEnd(14),
        result.req_per_sec.toFixed(0).padStart(8),
        result.avg_ms.toFixed(3).padStart(8),
        result.p50_ms.toFixed(3).padStart(8),
        result.p95_ms.toFixed(3).padStart(8),
        result.p99_ms.toFixed(3).padStart(8),
        result.wall_ms.toFixed(1).padStart(9),
    ].join("  ")
}

const payload = makePayload(itemCount)
const payloadBytes = new TextEncoder().encode(JSON.stringify(payload)).byteLength
const requestInit = acceptEncoding
    ? { headers: { "Accept-Encoding": acceptEncoding } }
    : undefined

const jsonClient = new Client(baseURL, { transport: "json", requestInit })
const wireJSONClient = new Client(baseURL, { transport: "wire-json-strict", disableCapabilityPreflight: true, requestInit })
const wireBinaryClient = new Client(baseURL, { transport: "binary-strict", disableCapabilityPreflight: true, requestInit })
const autoClient = new Client(baseURL, { transport: "auto", requestInit })

for (const client of [jsonClient, wireJSONClient, wireBinaryClient, autoClient]) {
    for (let i = 0; i < 150; i++) {
        await client.bench.Echo(payload)
    }
}

const results = [
    await bench("json", jsonClient, payload),
    await bench("wire-json", wireJSONClient, payload),
    await bench("wire-binary", wireBinaryClient, payload),
    await bench("auto-cached", autoClient, payload),
]
const byteSamples = [
    await measureBytes("json", jsonClient, payload),
    await measureBytes("wire-json", wireJSONClient, payload),
    await measureBytes("wire-binary", wireBinaryClient, payload),
    await measureBytes("auto", autoClient, payload),
]

console.log(JSON.stringify({
    base_url: baseURL,
    iterations,
    concurrency,
    items: itemCount,
    accept_encoding: acceptEncoding ?? "fetch-default",
    payload_bytes_json: payloadBytes,
    results,
    byte_samples: byteSamples,
}, null, 2))

console.log("")
console.log("mode             req/s    avg ms    p50 ms    p95 ms    p99 ms   wall ms")
for (const result of results) {
    console.log(formatRow(result))
}
