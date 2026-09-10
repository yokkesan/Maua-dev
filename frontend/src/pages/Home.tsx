import logoAuma from "../assets/logo/logo-auma-black.svg"
import topLifeStyle from "../assets/top/top-life-style.jpg"

export default function Home() {
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
          </div>

          <div className="top-page__hero-image">
            <img src={topLifeStyle} alt="Life Style" />
          </div>
        </div>
      </section>
    </main>
  )
}