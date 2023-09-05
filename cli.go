package greenlight

type CLI struct {
	Config string `help:"config file path or URL" short:"c" required:"true" default:"greenlight.yaml"`
}
