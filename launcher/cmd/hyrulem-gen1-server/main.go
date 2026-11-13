// Development server: requires a separately implemented native game adapter.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hyrulelauncher/internal/gen1"
	"io"
	"os"
	"os/signal"
)

func run() error {
	address := flag.String("listen", "0.0.0.0:5051", "IP local e porta TCP")
	catalog := flag.String("catalog", "", "catálogo JSON validado pelo servidor (obrigatório)")
	pvp := flag.Bool("pvp", true, "permitir ataques entre jogadores")
	simulation := flag.Bool("allow-simulation", false, "aceitar adaptadores sintéticos para desenvolvimento; não executa BOTW")
	flag.Parse()
	if *catalog == "" {
		return fmt.Errorf("informe --catalog; consulte docs/gen1/README.md")
	}
	f, err := os.Open(*catalog)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1024*1024))
	decoder.DisallowUnknownFields()
	var definitions gen1.Catalog
	if err = decoder.Decode(&definitions); err != nil {
		return err
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return fmt.Errorf("dados adicionais no catálogo")
	}
	s, err := gen1.Listen(*address, gen1.Config{Catalog: definitions, AllowPVP: *pvp, AllowSimulation: *simulation})
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Printf("HyruleM Gen1 DEVELOPMENT · protocolo %d · %s\n", gen1.Protocol, s.Addr())
	fmt.Println("Requer adaptador nativo ainda não implementado no Savage Wind oficial. O launcher 0.4.x usa outro servidor.")
	if *simulation {
		fmt.Println("SIMULAÇÃO habilitada: aceita clientes sintéticos; isso não cria atores em BOTW.")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
