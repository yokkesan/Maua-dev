import topLifeStyle from "../assets/top/top-life-style.jpg"
import topFood from "../assets/top/top-food.jpg"
import topBeauty from "../assets/top/top-beauty.jpg"
import topFashion from "../assets/top/top-fashion.jpg"

export const heroSlides = [
    {
        number: "01",
        title: "Life Style",
        image: topLifeStyle,
        description:
            "インテリアや植物、ステーショナリー、本など、日々の生活に密着したアイテムたち。こだわりを大切に、生活を豊かにしてくれます。",
    },
    {
        number: "02",
        title: "Food",
        image: topFood,
        description:
            "生産者や製造元のセンスの良さや想いが詰まった食品ブランドが勢揃い！質が良くこだわりのあるフードやドリンク、加工品、その他の商材を幅広くセレクトしています。",
    },
    {
        number: "03",
        title: "Beauty&Health",
        image: topBeauty,
        description:
            "全ての人が求める美と健康。時代に即した上質で機能的な必須のアイテムが充実。オーガニックからメディカルまで、さまざまな商材が揃います。",
    },
    {
        number: "04",
        title: "Fashion",
        image: topFashion,
        description:
            "フォーマルからカジュアルまで、多様性に富んだ多彩なアイテムの数々。シーズン毎の新作から定番アイテムまで、幅広く揃います。",
    },
] as const