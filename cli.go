package greenlight

type CLI struct {
	Config string `help:"config file path or URL(http,https,file,s3)" short:"c" required:"true" default:"greenlight.yaml"`
	Debug  bool   `help:"debug mode" short:"d" default:"false"`
}
