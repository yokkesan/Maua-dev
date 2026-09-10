export type TopMenuButton = {
    type: "register" | "use" | "demo" | "about" | "upgrade"
    label: string
    small?: string
    campaign?: string
    path?: string
    modal?: "_use" | "_demo" | "_about-auma"
}

const brandUserButtons: TopMenuButton[] = [
    {
        type: "about",
        label: "AUMA について",
        small: "（料金プラン）",
        modal: "_about-auma",
    },
    {
        type: "use",
        label: "ご利用ガイド",
        modal: "_use",
    },
    {
        type: "demo",
        label: "デモを見る",
        modal: "_demo",
    },
    {
        type: "upgrade",
        label: "アップグレードする",
        campaign: "(BASICプラン ￥4,950/月)",
        path: "/admin/brands",
    },
]

const guestButtons: TopMenuButton[] = [
    {
        type: "register",
        label: "無料でAUMAに登録",
        path: "/registration",
    },
    {
        type: "use",
        label: "ご利用ガイド",
        modal: "_use",
    },
    {
        type: "demo",
        label: "デモを見る",
        modal: "_demo",
    },
    {
        type: "about",
        label: "AUMA について",
        small: "（料金プラン）",
        modal: "_about-auma",
    },
]

export const getTopMenuButtons = (isBrandUser: boolean): TopMenuButton[] => {
    return isBrandUser ? brandUserButtons : guestButtons
}