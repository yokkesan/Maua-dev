import { useState } from "react"
import { Link } from "react-router-dom"

type SubMenuName = "about" | "guide" | "terms"

export default function MenuLinkListStatic() {
    const [openSubMenu, setOpenSubMenu] = useState<SubMenuName | null>(null)

    const toggleSubMenu = (name: SubMenuName) => {
        setOpenSubMenu((current) => (current === name ? null : name))
    }

    return (
        <>
            <li>
                <Link to="/registration">
                    <span>新規登録（無料）</span>
                </Link>
            </li>

            <li>
                <Link to="/login">
                    <span>ログイン</span>
                </Link>
            </li>

            <li className="_about">
                <Link to="/brand-list">
                    <span>ブランド検索</span>
                </Link>
            </li>

            <li className="_about">
                <button
                    type="button"
                    className="js-toggle-on"
                    onClick={() => toggleSubMenu("about")}
                    aria-expanded={openSubMenu === "about"}
                >
                    <span>AUMAについて（料金プラン）</span>
                </button>

                <ul className={`menu_lv2-nest${openSubMenu === "about" ? " _open" : ""}`}>
                    <li>
                        <Link to="/brand/about">
                            <span>ブランド様用</span>
                        </Link>
                    </li>
                    <li>
                        <Link to="/shop/about">
                            <span>ショップ様用</span>
                        </Link>
                    </li>
                </ul>
            </li>

            <li>
                <button
                    type="button"
                    className="js-toggle-on"
                    onClick={() => toggleSubMenu("guide")}
                    aria-expanded={openSubMenu === "guide"}
                >
                    <span>ご利用ガイド</span>
                </button>

                <ul className={`menu_lv2-nest${openSubMenu === "guide" ? " _open" : ""}`}>
                    <li>
                        <Link to="/brand/guide">
                            <span>ブランド様用</span>
                        </Link>
                    </li>
                    <li>
                        <Link to="/shop/guide">
                            <span>ショップ様用</span>
                        </Link>
                    </li>
                </ul>
            </li>

            <li>
                <Link to="/faq">
                    <span>よくある質問</span>
                </Link>
            </li>

            <li>
                <Link to="/company">
                    <span>運営会社情報</span>
                </Link>
            </li>

            <li>
                <Link to="/privacy">
                    <span>プライバシーポリシー</span>
                </Link>
            </li>

            <li>
                <button
                    type="button"
                    className="js-toggle-on"
                    onClick={() => toggleSubMenu("terms")}
                    aria-expanded={openSubMenu === "terms"}
                >
                    <span>利用規約</span>
                </button>

                <ul className={`menu_lv2-nest${openSubMenu === "terms" ? " _open" : ""}`}>
                    <li>
                        <Link to="/brand/rule">
                            <span>ブランド様用</span>
                        </Link>
                    </li>
                    <li>
                        <Link to="/shop/rule">
                            <span>ショップ様用</span>
                        </Link>
                    </li>
                </ul>
            </li>

            <li>
                <a href="mailto:support@auma.jp?subject=【AUMAサポート】問い合わせ&body=ご記入ください">
                    <span>お問合せ</span>
                </a>
            </li>
        </>
    )
}