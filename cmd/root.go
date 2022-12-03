package cmd

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/pandada8/ic99-web/pkg/charger"
	"github.com/pandada8/ic99-web/pkg/web"
	"github.com/spf13/cobra"
)

var (
	LISTEN string
)

var rootCmd = &cobra.Command{
	Use:   "ic99-web",
	Short: "A Web Daemon for IC99 Charger",
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
		r := gin.Default()
		r.GET("/api/ws", func(c *gin.Context) {
			web.WebSocketHandler(c.Writer, c.Request)
		})
		go charger.StartLoop()
		r.Run(LISTEN)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&LISTEN, "listen", ":8080", "listen address")
}

func getChargerStats(c *gin.Context) {
}
