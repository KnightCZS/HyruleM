package main

import (
	"flag"
	"fmt"
	mp "hyrulelauncher/internal/multiplayer"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:5050", "IP:porta para escutar")
	x := flag.Float64("spawn-x", 3437.5, "X do encontro")
	y := flag.Float64("spawn-y", 235.9, "Y do encontro")
	z := flag.Float64("spawn-z", 2101.5, "Z do encontro")
	flag.Parse()
	server, e := mp.Listen(*listen, mp.ServerConfig{Spawn: mp.Vec3{X: *x, Y: *y, Z: *z}, Log: func(s string) { fmt.Println(s) }})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer server.Close()
	fmt.Println("HyruleM · protótipo visual para 2 jogadores ·", server.Addr())
	fmt.Println("Avatares provisórios; posições e nomes. Ctrl+C para encerrar.")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
}
