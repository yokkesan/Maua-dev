import { Link } from "react-router-dom"
import { Swiper, SwiperSlide } from "swiper/react"
import { Autoplay, Pagination } from "swiper/modules"

import "swiper/css"
import "swiper/css/pagination"

import logoAuma from "../assets/logo/logo-auma-black.svg"
import { heroSlides } from "../data/heroSlides"
import { getTopMenuButtons } from "../data/topMenuButtons"

export default function Home() {
  const isBrandUser = false
  const menuButtons = getTopMenuButtons(isBrandUser)

  return (
    <main className="top-page">
      <section className="top-page__first-view">
        <div className="top-page__inner">
          <div className="top-page__info">
            <div className="top-page__logo">
              <img src={logoAuma} alt="AUMA" />
            </div>

            <div className="top-page__text">
              <h1>衣・食・住・美</h1>
              <p>AUMAはハイセンスなクリエイターとバイヤーが集まり繋がることができるBtoB情報検索サービス。</p>
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
    </main>
  )
}