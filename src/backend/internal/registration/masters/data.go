package masters

// Goコードで管理する固定マスターデータ。
// IDは一度運用を開始したら変更しないこと。

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

// 都道府県

// 募集区分
var salesRecruitments = []MasterItem{}

// 販売形態
var salesStyles = []MasterItem{}

// 注文条件
var orders = []MasterItem{}

// 支払方法
var payRules = []MasterItem{}
