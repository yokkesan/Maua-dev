import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Swiper, SwiperSlide } from "swiper/react"
import { Autoplay, Pagination } from "swiper/modules"

import "swiper/css"
import "swiper/css/pagination"

import MenuAssets from "../components/MenuAssets"

import logoAuma from "../assets/logo/logo-auma-black.svg"
import { heroSlides } from "../data/heroSlides"
import { getTopMenuButtons } from "../data/topMenuButtons"
import { fetchTopNotice, type TopNotice } from "../api/top"

export default function Home() {
  const isBrandUser = false
  const menuButtons = getTopMenuButtons(isBrandUser)

  const [notice, setNotice] = useState<TopNotice | null>(null)

  useEffect(() => {
    const controller = new AbortController()

    fetchTopNotice(controller.signal)
      .then(setNotice)
      .catch((error) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return
        }

        console.error("Failed to fetch top notice", error)
      })

    return () => controller.abort()
  }, [])

  return (
    <>
      <MenuAssets />

      <main className="top-page">
        <section className="top-page__first-view">
          <div className="top-page__inner">
            <div className="top-page__info">
              <div className="top-page__logo">
                <img src={logoAuma} alt="AUMA" />
              </div>

              <div className="top-page__text">
                <h1>衣・食・住・美</h1>
                <p>
                  AUMAはハイセンスなクリエイターとバイヤーが集まり繋がることができるBtoB情報検索サービス。
                </p>
              </div>

              <div className="top-page__menu">
                {menuButtons.map((button) => (
                  <div
                    key={button.type}
                    className={`btn_link-wrap${button.type === "upgrade" ? " _upgrade" : ""}`}
                  >
                    {"path" in button ? (
                      <Link to={button.path} className="btn_link">
                        <span className="inner">
                          <span>{button.label}</span>

                          {"small" in button && button.small && (
                            <small>{button.small}</small>
                          )}

                          {"campaign" in button && button.campaign && (
                            <small>{button.campaign}</small>
                          )}
                        </span>
                      </Link>
                    ) : (
                      <button type="button" className="btn_link">
                        <span className="inner">
                          <span>{button.label}</span>

                          {"small" in button && button.small && (
                            <small>{button.small}</small>
                          )}

                          {"campaign" in button && button.campaign && (
                            <small>{button.campaign}</small>
                          )}
                        </span>
                      </button>
                    )}

                    <div className="bg-element" aria-hidden="true" />
                  </div>
                ))}
              </div>
            </div>

            <div className="top-page__hero">
              <Swiper
                modules={[Autoplay, Pagination]}
                slidesPerView={1}
                speed={1000}
                pagination={{ clickable: true }}
                autoplay={{
                  delay: 5000,
                  disableOnInteraction: false,
                }}
                loop
              >
                {heroSlides.map((slide) => (
                  <SwiperSlide key={slide.number}>
                    <div className="top-page__hero-slide">
                      <div className="top-page__hero-image">
                        <img src={slide.image} alt={slide.title} />
                      </div>

                      <div className="top-page__hero-detail">
                        <div className="top-page__hero-number">
                          <span className="top-page__hero-bar" />
                          <span>{slide.number}</span>
                        </div>

                        <h2>{slide.title}</h2>
                        <p>{slide.description}</p>
                      </div>
                    </div>
                  </SwiperSlide>
                ))}
              </Swiper>
            </div>
          </div>
        </section>

        {notice && (
          <section className="top-page_brand-list common">
            <div className="section-inner">
              <div className="list_note-link-v2-wrap">
                <h2 className="list_note-link-v2-title">お知らせ</h2>

                <div className="list_note-link-v2-list">
                  <ul>
                    <li className="list_note-link-v2-list-item">
                      <a
                        href={notice.url}
                        className="list_note-link-v2-list-item-body"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <div className="list_note-link-v2-list-item-date">
                          {new Intl.DateTimeFormat("ja-JP", {
                            year: "numeric",
                            month: "2-digit",
                            day: "2-digit",
                          }).format(new Date(notice.publishedAt))}
                        </div>

                        <div className="list_note-link-v2-list-item-detail">
                          <p className="list_note-link-v2-list-item-text">
                            {notice.title}
                          </p>

                          <div className="md_svg-icon">
                            <svg
                              width="100"
                              height="100"
                              viewBox="0 0 100 100"
                              fill="none"
                              xmlns="http://www.w3.org/2000/svg"
                              aria-hidden="true"
                            >
                              <path
                                d="M40.4225 29.1539C26.6426 29.1539 19 29.1539 19 29.1539V84H73.8461V60.0157"
                                stroke="black"
                                strokeWidth="4"
                                strokeLinecap="square"
                              />
                              <path
                                d="M58 22H78V42"
                                stroke="black"
                                strokeWidth="4"
                              />
                              <path
                                d="M77 23L34 66"
                                stroke="black"
                                strokeWidth="4"
                              />
                            </svg>
                          </div>
                        </div>
                      </a>
                    </li>
                  </ul>

                  <div className="list_note-link-v2-logo">
                    <svg
                      width="478"
                      height="104"
                      viewBox="0 0 478 104"
                      fill="none"
                      xmlns="http://www.w3.org/2000/svg"
                      aria-hidden="true"
                    >
                      <path
                        d="M61.5 8.99986C43.5 8.49986 18.3 9.49986 0 9.49986V103.1H26.2V32.2999C26.2 32.2999 49.5 31.4999 57.9 32.4999C65.8 33.3999 68.7 38.2999 69 48.4999C69.2 60.0999 69 66.1999 69 103.1H95.2C95.2 103.1 95.5 60.8999 95.2 46.1999C94.8 20.0999 85.7 9.59986 61.5 8.99986Z"
                        fill="black"
                      />
                      <path
                        d="M182.7 8.7002C156.8 8.7002 135.7 29.9002 135.7 56.0002C135.7 82.1002 156.8 103.3 182.7 103.3C208.6 103.3 229.7 82.1002 229.7 56.0002C229.7 29.9002 208.6 8.7002 182.7 8.7002ZM182.7 80.7002C169.1 80.7002 158.1 69.6002 158.1 55.9002C158.1 42.2002 169.1 31.1002 182.7 31.1002C196.3 31.1002 207.3 42.2002 207.3 55.9002C207.3 69.7002 196.3 80.7002 182.7 80.7002Z"
                        fill="black"
                      />
                      <path
                        d="M477.3 56.0002C477.3 29.9002 456.2 8.7002 430.3 8.7002C404.4 8.7002 383.3 29.9002 383.3 56.0002C383.3 71.3002 390.3 84.4002 401.2 93.1002C408.8 99.1002 419.4 103.3 433.6 103.3C440.5 103.3 458.4 101.4 471.3 87.1002L460.7 72.9002C456.2 76.9002 444.9 82.5002 435.9 82.5002C424.5 82.5002 417.6 80.3002 412.6 75.6002C409.5 72.8002 407.3 68.5002 406.3 63.2002H476.6C477 60.8002 477.3 58.5002 477.3 56.0002ZM406.5 47.4002C409.1 36.3002 416.9 28.1002 430.2 28.1002C444.1 28.1002 451.4 36.9002 453.8 47.4002H406.5Z"
                        fill="black"
                      />
                      <path
                        d="M318.7 0H294.2V26.3H269V49.3H294.2V76.6H318.7V49.3H343.9V26.3H318.7V0Z"
                        fill="black"
                      />
                    </svg>
                  </div>
                </div>
              </div>
            </div>
          </section>
        )}
      </main>
    </>
  )
}