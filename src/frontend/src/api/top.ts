export type TopNotice = {
    title: string
    url: string
    publishedAt: string
}

export async function fetchTopNotice(signal?: AbortSignal): Promise<TopNotice> {
    const response = await fetch("/api/v1/top/notice", { signal })

    if (!response.ok) {
        throw new Error(`Failed to fetch top notice: ${response.status}`)
    }

    return response.json() as Promise<TopNotice>
}