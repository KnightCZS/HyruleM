package main

import (
	"flag"
	"fmt"
	mp "hyrulelauncher/internal/multiplayer"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:5050", "IP:porta para escutar")
	x := flag.Float64("spawn-x", 3437.5, "X do encontro")
	y := flag.Float64("spawn-y", 235.9, "Y do encontro")
	z := flag.Float64("spawn-z", 2101.5, "Z do encontro")
	items := flag.Bool("shared-items", true, "Experimento: itens e armas nas proximidades dos jogadores; use --shared-items=false para desligar")
	dedicated := flag.Bool("dedicated-world", true, "Servidor decide a autoridade por entidade e mantém horário e clima")
	state := flag.String("world", "hyrulem-world.json", "Arquivo persistente do mundo dedicado")
	day := flag.Duration("day-length", 24*time.Minute, "Duração de um dia completo")
	flag.Parse()
	server, e := mp.Listen(*listen, mp.ServerConfig{DedicatedWorld: *dedicated, StatePath: *state, DayLength: *day, SharedItems: *items, Spawn: mp.Vec3{X: *x, Y: *y, Z: *z}, Log: func(s string) { fmt.Println(s) }})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("HyruleM · sessão para 2 jogadores ·", server.Addr())
	fmt.Println("Mundo dedicado, autoridade por entidade e estado persistente. Ctrl+C para gravar e encerrar.")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case <-quit:
	case err := <-server.Errors():
		fmt.Fprintln(os.Stderr, err)
		server.Close()
		os.Exit(1)
	}
	if err := server.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
