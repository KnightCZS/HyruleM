package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hyrulelauncher/internal/launcher"
	"hyrulelauncher/internal/nativebridge"
	"os"
	"path/filepath"
	"time"
)

func run() error {
	config := flag.String("config", "", "config.json do launcher; padrão: configuração já salva")
	slot := flag.Int("slot", 1, "0 = sandbox, 1/2 = instância multiplayer")
	port := flag.Int("port", 0, "porta de controle local; 0 procura no log da instância")
	out := flag.String("out", "HyruleM-native-report.json", "arquivo de diagnóstico a criar (não sobrescreve)")
	flag.Parse()
	if *config == "" {
		dir, err := launcher.AppDir()
		if err != nil {
			return err
		}
		*config = filepath.Join(dir, "config.json")
	}
	cfg, err := launcher.LoadConfig(*config)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	report, err := nativebridge.Probe(ctx, cfg, *slot, *port)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("Relatório criado:", *out)
	if !report.NativeReady {
		fmt.Println(report.NativeError)
	}
	fmt.Println("Somente leitura. Não contém dump, saves ou inventário.")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
