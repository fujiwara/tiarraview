package tiarraview

type Config struct {
	DBFile     string `name:"dbfile" default:"./db/database.sqlite3"`
	SchemaFile string `name:"schemafile"`
	Server     struct {
		Addr string `name:"addr" default:":8080"`
	} `cmd:"" help:"run web view server"`
	Import struct {
		SrcDir string `name:"src-dir" required:""`
	} `cmd:"" help:"import log files to database"`
	Init struct {
	} `cmd:"" help:"initialize database"`
}
