package masters

// Goコードで管理する固定マスターデータ。
// IDは一度運用を開始したら変更しないこと。

// 地方
var areaRegions = []AreaRegion{
	{ID: 1, Name: "北海道"},
	{ID: 2, Name: "東北"},
	{ID: 3, Name: "関東"},
	{ID: 4, Name: "中部"},
	{ID: 5, Name: "近畿"},
	{ID: 6, Name: "中国"},
	{ID: 7, Name: "四国"},
	{ID: 8, Name: "九州・沖縄"},
}

// 募集区分
var salesRecruitments = []MasterItem{
	{ID: 1, Name: "ショップ"},
	{ID: 2, Name: "販売代行"},
	{ID: 3, Name: "フランチャイズ"},
	{ID: 4, Name: "ECサイトのみ"},
	{ID: 5, Name: "ショップ+ECサイト"},
	{ID: 6, Name: "店舗内スペース"},
	{ID: 7, Name: "テナント（商業施設）"},
	{ID: 8, Name: "セールスレップ(営業代行)"},
}

// 販売形態
var salesStyles = []MasterItem{
	{ID: 1, Name: "買取"},
	{ID: 2, Name: "委託"},
	{ID: 3, Name: "一部委託"},
	{ID: 4, Name: "受注会"},
	{ID: 5, Name: "パッケージ（セット）販売"},
	{ID: 6, Name: "ポップアップ販売（人あり）"},
	{ID: 7, Name: "ポップアップ販売（人なし）"},
	{ID: 8, Name: "その他"},
}

// 注文条件
var orders = []MasterItem{
	{ID: 1, Name: "小ロット"},
	{ID: 2, Name: "ODM"},
	{ID: 3, Name: "OEM"},
	{ID: 4, Name: "サンプル販売"},
	{ID: 5, Name: "なし"},
}

// 支払方法
var payRules = []MasterItem{
	{ID: 1, Name: "COD（現金先払い）"},
	{ID: 2, Name: "初回COD（初回現金先払い）"},
	{ID: 3, Name: "締め払い"},
	{ID: 4, Name: "プリペイドオーダー"},
	{ID: 5, Name: "その他（弊社規定に準じる）"},
}
