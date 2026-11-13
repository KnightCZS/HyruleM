package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hyrulelauncher/internal/nativeassets"
	"os"
	"path/filepath"
)

func run() error {
	romfs := flag.String("romfs", "", "pasta romfs extraída do seu BOTW Switch 1.9.0")
	out := flag.String("out", "", "nova pasta para o overlay privado experimental")
	flag.Parse()
	if *romfs == "" || *out == "" {
		return fmt.Errorf("informe --romfs e --out; os originais permanecem intactos")
	}
	if _, err := os.Stat(*out); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("a pasta --out deve ser nova")
	}
	read := func(path string) ([]byte, error) { return os.ReadFile(filepath.Join(*romfs, filepath.FromSlash(path))) }
	title, err := read("Pack/TitleBG.pack")
	if err != nil {
		return err
	}
	info, err := read(nativeassets.InfoPath)
	if err != nil {
		return err
	}
	rstb, err := read(nativeassets.RSTBPath)
	if err != nil {
		return err
	}
	files, err := nativeassets.BuildRemote(title, info, rstb)
	if err != nil {
		return err
	}
	parent := filepath.Dir(*out)
	if err = os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".hyrulem-native-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	manifest := struct {
		Actor        string            `json:"actor"`
		Experimental bool              `json:"experimental"`
		Files        map[string]string `json:"sha256"`
	}{nativeassets.Actor, true, map[string]string{}}
	for name, data := range files {
		path := filepath.Join(stage, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		manifest.Files[name] = hex.EncodeToString(sum[:])
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "hyrulem-native-assets.json"), data, 0600); err != nil {
		return err
	}
	if err = os.Rename(stage, *out); err != nil {
		return err
	}
	fmt.Println("Overlay privado experimental criado. Não distribua os arquivos gerados. Requer o runtime HyruleM modificado.")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
