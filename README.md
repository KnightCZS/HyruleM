# HyruleM

Prévia experimental [0.5.0-preview.3](https://github.com/KnightCZS/HyruleM/releases/tag/v0.5.0-preview.3): corrige instalação e seleção do modo nativo após os relatórios da issue #4 mostrarem ABI 1 ainda em uso. `Iniciar-Nativo.cmd` confere e atualiza o runtime configurado; o EXE do pacote também seleciona o modo nativo automaticamente. [Instruções](native/README.md). A visibilidade em gameplay ainda requer validação com ABI 2.

Launcher para explorar BOTW no Savage Wind, com sandbox e um primeiro protótipo **visual** de multiplayer para duas pessoas. Windows x64; Savage Wind `v0.5.3-alpha.5`, BOTW Switch 1.9.0 já preparado pelo proprietário.

## Estado da versão 0.4.2

**Atualização experimental 0.5.0-preview.2:** corrige o callback que impedia o ator remoto de ser criado, retira o teleporte automático ao conectar e adiciona convergência de vida dos monstros próximos. Ambos os jogadores e o servidor precisam do protocolo 3. Veja [instalação, limites e validação](native/README.md). A renderização dos dois Links ainda exige confirmação em gameplay; PvP, coleta e IA compartilhada continuam pendentes. O restante desta seção descreve o protótipo visual estável 0.4.2.

**Desenvolvimento da geração nativa:** esta branch contém as regras autoritativas e agora um [adaptador experimental de presença nativa](native/README.md), com build próprio do runtime, criação/movimento/remoção de ator e geração local dos recursos. Esse caminho ainda não foi validado com dois jogos e não implementa combate/coleta/IA completos. Não é a Gen1 jogável. O pacote estável 0.4.2 continua sendo o protótipo visual descrito abaixo. Veja também [Gen1: estado e diagnóstico](docs/gen1/README.md).

Hotfix dos [issues #1](https://github.com/KnightCZS/HyruleM/issues/1) e [#2](https://github.com/KnightCZS/HyruleM/issues/2): leitura de fluxos EVFL válidos sem entradas e preparação automática do filtro antes de conectar. [Notas 0.4.2](releases/v0.4.2.md).

- Menu com nome, IP, porta TCP, hospedagem e dois clientes no mesmo PC.
- Posição real e câmera lidas pelo controle local do Savage Wind.
- Movimento retransmitido por servidor próprio, interpolação e nome sobre o avatar.
- Avatar humanoide procedural desenhado em uma janela transparente sobre cada jogo.
- Saves, configurações, caches, logs e portas de controle separados por instância.
- Campanha offline, sandbox e filtro experimental de história da versão 0.3 preservados.

**O jogador remoto ainda não é um ator nativo de BOTW.** A imagem é uma projeção externa: atravessa visualmente paredes, não colide e usa animação aproximada. O teste inicial deve ocorrer em Hateno, no mundo aberto, em modo janela/SDR. Não há sincronização de monstros, inventário, clima, horário, missões ou mapas internos. A integração com duas sessões reais de BOTW ainda precisa de validação em máquina com jogo e GPU compatível.

## Testar

Use o pacote Windows 0.4.2 ou os artefatos de [Actions](../../actions). O pacote de atualização reaproveita sua instalação e os caminhos salvos pelo launcher anterior; não inclui jogo nem o runtime.

1. Feche o jogo e abra `HyruleLauncher.exe`. Confirme que seu sandbox já funciona.
2. Clique em **Multiplayer · 2 jogadores**.
3. Escolha a interface do seu computador (ou Todas as interfaces), informe uma porta TCP livre e clique em **Hospedar servidor**. O endereço para conectar aparece no menu. Para testar neste PC com Todas as interfaces, também pode usar `127.0.0.1`.
4. Escolha os nomes e conecte o cliente 1 e o cliente 2. Cada botão abre sua própria instância.
5. Em cada jogo escolha **Continuar**. Pare o personagem e aguarde o teleporte automático para posições próximas em Hateno.
6. Clique em **Organizar 2 janelas**. Mova um personagem e gire a câmera do outro para observá-lo. O teclado controla o jogo em foco.

Para dois PCs, hospede apenas em um; no outro informe o IP desse PC e a mesma porta. Com Radmin, selecione o adaptador Radmin do anfitrião e use esse IP no convidado. Autorize a porta TCP escolhida no firewall do anfitrião. Em redes diferentes, use uma VPN entre os PCs ou configure o encaminhamento da porta. O protótipo não tem autenticação ou criptografia: use com pessoas de confiança.

[Baixar Releases](https://github.com/KnightCZS/HyruleM/releases) · [Alterações, adições e correções](CHANGELOG.md)

[Análise das correções 0.4.1](docs/CORRECOES_0.4.1.md) · [Instruções e diagnóstico](docs/MULTIPLAYER.md) · [Pesquisa e decisões técnicas](docs/PESQUISA_MULTIPLAYER.md) · [Validação](docs/VALIDACAO_0.4.md)

## Compilar

Go 1.23 ou superior, sem bibliotecas Go externas:

```sh
cd launcher
go test -race ./internal/...
```

No Windows execute `launcher/build.cmd`; gera `launcher/dist/HyruleLauncher.exe` e `HyruleM-Server.exe`. O ícone e o manifesto compilados são próprios e estão incluídos. Regenerá-los exige Python e Pillow (`tools/build_resources.py`).

Servidor independente, caso não queira hospedar no launcher:

```sh
HyruleM-Server.exe --listen 0.0.0.0:5050
```

O servidor visual não executa BOTW e não contém os arquivos do jogo. Seu protocolo transporta somente presença e movimento. A geração nativa em desenvolvimento usa um servidor/protocolo separado, com regras de mundo e integração obrigatória com atores reais.

## Licenças e arquivos

O `LICENSE` original da raiz (Unlicense) foi preservado. O componente `launcher/` mantém sua licença MIT própria, incluindo a implementação do protótipo. Savage Wind e Go conservam suas licenças; veja `launcher/THIRD_PARTY_NOTICES.md` e `launcher/GO_LICENSE.txt`. Nenhum código ou BNP do Milk Bar foi incorporado. Nenhum executável recompilado do jogo, ROM, dump, fonte de console ou asset Nintendo deve ser versionado.
