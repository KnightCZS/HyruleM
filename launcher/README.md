> Correções 0.4.1: [histórico](../CHANGELOG.md), [release](../releases/v0.4.1.md) e [guia](../docs/MULTIPLAYER.md).

> Versão 0.4.0: menu multiplayer visual experimental para dois jogadores. Consulte [README principal](../README.md), [guia](../docs/MULTIPLAYER.md) e [limitações/validação](../docs/VALIDACAO_0.4.md). O avatar remoto ainda não é um ator nativo do jogo.

# Hyrule Launcher 0.3.0 — Windows x64

Launcher nativo para campanha offline e sandbox com Savage Wind. O executável não exige Python, Node ou Go para funcionar.

## Atualizar a instalação que já funcionou

Feche o jogo e o launcher antigo. Extraia o ZIP 0.3.0 e abra `HyruleLauncher.exe`. Seus caminhos e seu sandbox existente serão reaproveitados.

Na seção Sandbox, deixe **Campanha bloqueada · secundárias e santuários**, clique em **Jogar sandbox** e aguarde a geração inicial do filtro. Na recomp, escolha **Continuar**. Não é necessário criar outro save ou recompilar o jogo.

Leia [FILTRO_CAMPANHA.md](FILTRO_CAMPANHA.md) para o filtro e sua recuperação, e [SANDBOX.md](SANDBOX.md) para saves, voo, teleporte, clima e itens. A camada de campanha 0.3 é experimental e ainda precisa de validação dentro do BOTW real.

O pacote de atualização contém executável, fontes e documentação. O pacote completo também acompanha o ZIP oficial da recomp. Nenhum deles contém BOTW ou saves de jogadores.

## Primeira instalação

1. Extraia o pacote completo inteiro e abra HyruleLauncher.exe.
2. Use **Instalar Savage Wind · pacote incluído**. O launcher verifica o SHA-256 antes de extrair.
3. Use **Preparar jogo** e selecione, no Savage Wind, os arquivos da sua cópia Switch 1.9.0 com `exefs` e `romfs`.
4. Aguarde a preparação e compilação inicial. Depois use **Jogar campanha**.
5. Para o sandbox, crie primeiro um save normal que funcione nessa instalação e feche o jogo. O launcher usa uma cópia independente.

A preparação pode iniciar o jogo diretamente. Enquanto a recomp estiver aberta, o launcher acompanha a sessão; após fechá-la, aguarde a liberação de uma nova execução.

## Instalação Savage Wind existente

Selecione savagewind.exe em Programa da recomp e confira a pasta de dados. A raiz deve conter `extracted` e os dados normais da instalação. O filtro usa os arquivos em `extracted/romfs`; instalações antigas que mantenham apenas romfs.bin precisam disponibilizar os arquivos extraídos compatíveis.

As pastas de save e de overlay do sandbox são definidas apenas para esse processo. A campanha normal não recebe o filtro. A recomp oficial e sua DLL gerada não são modificadas.

## Botões do sandbox

- **Jogar sandbox:** prepara a cópia inicial, se necessário, ou retoma a existente; aplica o perfil de missões escolhido.
- **Novo + backup:** cria outro sandbox da base e preserva o anterior.
- **Atualizar filtro:** força a geração da camada de campanha sem recriar o save.
- **Escolher save base / Usar campanha atual:** escolhe a origem usada ao criar um novo sandbox.
- **Ferramentas / Aplicar / Teleportar:** comandos locais para o jogo carregado.
- **Abrir logs:** reúne registros e um resumo do filtro.

O fechamento do launcher não encerra o jogo à força.

## Backend Wii U opcional

Uma instalação própria do [BotW-Native](https://github.com/ryderclanky/BotW-Native) pode ser selecionada para campanha. Ela usa a pasta Wii U v208 / 1.5.0 com `code/U-King.rpx`, `content` e `meta`. Seu setup e dependências são os desse projeto. O sandbox e o filtro desta entrega exigem Savage Wind para Switch.

## Dados e código

A configuração fica em `%LOCALAPPDATA%/HyruleLauncher/config.json`, com backup da configuração anterior. Saves e filtro ficam na pasta de dados da recomp selecionada. O filtro é gerado localmente a partir dos arquivos do jogador e não precisa ser enviado a outra máquina.

A pasta `fontes` contém o projeto Go. Com Go 1.23 ou superior no Windows, execute `build.cmd`. Os recursos Windows já estão incluídos; Pillow só é necessário para regenerar o ícone e manifesto.

Consulte [ANALISE_SANDBOX.md](ANALISE_SANDBOX.md), [VALIDACAO.md](VALIDACAO.md) e [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Projeto próprio sob MIT; o runtime oficial mantém suas licenças.
