// Package web 把模板與靜態資源編進 binary。
//
// 為什麼要 embed：Docker image 只需要放一個執行檔，不用 COPY 一堆目錄，
// 也不會發生「本機跑得好好的、上了 Cloud Run 說找不到檔案」這種事。
//
// 注意 go:embed 的路徑不能往上跳（不能有 ..），所以這個檔案必須放在 web/ 底下，
// 而不是放在 internal/view/。
package web

import "embed"

// Templates 是 web/templates 整包。
// all: 前綴會連底線或點開頭的檔案一起收，避免漏掉。
//
//go:embed all:templates
var Templates embed.FS

// Static 是 web/static 整包——CSS、字型、圖示、前端 JS。
// 這些會由 Go 直接服務（本機開發），正式環境則由 Firebase Hosting 的 CDN 接手。
//
//go:embed all:static
var Static embed.FS
