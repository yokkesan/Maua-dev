import { useEffect, useState } from "react"
import { Link, useLocation } from "react-router-dom"

import logoAuma from "../assets/logo/logo-auma-black.svg"

import SvgIconLogin from "./svg/SvgIconLogin"
import SvgIconSnsFacebook from "./svg/SvgIconSnsFacebook"
import SvgIconSnsTwitter from "./svg/SvgIconSnsTwitter"
import SvgIconSnsInstagram from "./svg/SvgIconSnsInstagram"
import SvgIconSnsLine from "./svg/SvgIconSnsLine"

import MenuLinkListStatic from "./MenuLinkListStatic"

export default function MenuAssets() {
    const [openMenuHmbrg, setOpenMenuHmbrg] = useState(false)
    const location = useLocation()

    const closeMenuAll = () => {
        setOpenMenuHmbrg(false)
    }

    useEffect(() => {
        closeMenuAll()
    }, [location.pathname])

    useEffect(() => {
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                closeMenuAll()
            }
        }

        window.addEventListener("keydown", handleKeyDown)

        return () => {
            window.removeEventListener("keydown", handleKeyDown)
        }
    }, [])

    useEffect(() => {
        document.body.style.overflow = openMenuHmbrg ? "hidden" : ""

        return () => {
            document.body.style.overflow = ""
        }
    }, [openMenuHmbrg])

    return (
        <>
            <div className={`md_fixed-logo${openMenuHmbrg ? " _white" : ""}`}>
                <div className="logo">
                    <Link to="/" aria-label="AUMA トップページ">
                        <img src={logoAuma} alt="AUMA" />
                    </Link>
                </div>

                <div className="copy">
                    <span>さがす・つながる・ひろがる。</span>
                </div>
            </div>

            <div className="menu_hmbg-container">
                <button
                    type="button"
                    className={`hmbg-button${openMenuHmbrg ? " _open" : ""}`}
                    onClick={() => setOpenMenuHmbrg((current) => !current)}
                    aria-label={openMenuHmbrg ? "メニューを閉じる" : "メニューを開く"}
                    aria-expanded={openMenuHmbrg}
                    aria-controls="global-menu"
                >
                    <div className="inner _default">
                        <span />
                        <span />
                        <span />
                    </div>

                    <div className="inner _hover">
                        <span />
                        <span />
                        <span />
                    </div>

                    <div className="inner _close">
                        <span />
                        <span />
                    </div>
                </button>

                <nav
                    id="global-menu"
                    className="hmbg-menu"
                    aria-hidden={!openMenuHmbrg}
                >
                    <div className="md_scroll-content">
                        <div className="inner">
                            <div className="group">
                                <div className="heading_line">
                                    <p>
                                        <span>Menu</span>
                                    </p>
                                    <div className="line" />
                                </div>

                                <ul className="page">
                                    <MenuLinkListStatic />
                                </ul>
                            </div>

                            <div className="group">
                                <div className="heading_line">
                                    <p>
                                        <span>OFFICIAL SNS</span>
                                    </p>
                                    <div className="line" />
                                </div>

                                <ul className="sns">
                                    <li>
                                        <a
                                            href="https://www.facebook.com/profile.php?id=100088439449342"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                        >
                                            <div className="md_svg-icon">
                                                <SvgIconSnsFacebook />
                                            </div>
                                        </a>
                                    </li>

                                    <li>
                                        <a
                                            href="https://twitter.com/AUMA_official"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                        >
                                            <div className="md_svg-icon _small">
                                                <SvgIconSnsTwitter />
                                            </div>
                                        </a>
                                    </li>

                                    <li>
                                        <a
                                            href="https://www.instagram.com/auma.official/"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                        >
                                            <div className="md_svg-icon">
                                                <SvgIconSnsInstagram />
                                            </div>
                                        </a>
                                    </li>

                                    <li>
                                        <a
                                            href="https://lin.ee/s3UOfUY"
                                            target="_blank"
                                            rel="noopener noreferrer"
                                        >
                                            <div className="md_svg-icon">
                                                <SvgIconSnsLine />
                                            </div>
                                        </a>
                                    </li>
                                </ul>
                            </div>
                        </div>
                    </div>
                </nav>
            </div>

            <div className={`menu_search-container${openMenuHmbrg ? " _none" : ""}`}>
                <Link
                    to="/login"
                    className="md_icon-btn _menu-open"
                    aria-label="ログイン"
                >
                    <div className="inner">
                        <SvgIconLogin />
                    </div>
                </Link>
            </div>

            <div
                className={`md_curtain-container${openMenuHmbrg ? " _open" : ""}`}
                aria-hidden="true"
            >
                <div className="no1" />
                <div className="no2" />
                <div className="no3" />
            </div>
        </>
    )
}