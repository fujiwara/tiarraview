package tiarraview

type Config struct {
	DBFile string `name:"dbfile" default:"./db/database.sqlite3"`
	Server struct {
		Addr string `name:"addr" default:":8080"`
	} `cmd:""`
	Import struct {
		SrcDir string `name:"src-dir" required:""`
	} `cmd:""`
}
