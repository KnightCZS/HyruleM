# Geração 1: mundo e atores nativos (desenvolvimento)

**Estado em 13/11/2026: regras do servidor e contrato de integração implementados e testados; criação de atores nativos ainda não implementada. Esta branch não é uma versão jogável com dois Links nativos.** O launcher e a release estável 0.4.2 continuam com o protótipo visual externo.

O vídeo de 71 segundos enviado pelo usuário mostra o Link local andando em Hateno e abrindo seu inventário. Ele ajuda a confirmar o cenário de teste. Não constitui validação de criação, colisão ou combate de um segundo ator nativo.

## O que está no código

| Pedido | Implementado nesta branch | Falta para funcionar em BOTW |
|---|---|---|
| Dois Links com modelos e animações reais | Separação do ator local e remoto, planos de criação/remoção por mundo | Backend para criar um ator Link remoto, aplicar animação, equipamento e colisão |
| Vida e dano individuais | Estado privado por conexão, dano só na vítima e rejeição de dano ambiental em terceiros | Ler/aplicar vida na thread do jogo e interceptar dano antes de duplicá-lo |
| Inventário próprio | Inventário privado, equipar e descartar apenas itens próprios | Mapear itens nativos e reconciliar a bolsa sem tocar nos itens fora do catálogo |
| Arma coletada fica com quem pegou | Coleta atômica; uma disputa tem um único vencedor; registro repetido não recria o item | Interceptar coleta e remoção de ator, aguardar decisão do servidor |
| Links podem bater | Ataque corpo a corpo com distância, cooldown, dano e durabilidade definidos pelo servidor; PvP opcional | Evento real de acerto, animação, reação, hitbox e fórmula compatível com BOTW |
| Monstro troca para quem está mais perto | Seleção exata por distância 3D dentro do alcance, autoridade única e mudança de geração | Apontar IA para o ator correto e impedir a segunda cópia de simular/causar dano |

Os testes de rede usam duas conexões TCP reais com adaptadores sintéticos identificados como tais. Não executam o jogo. A flag `--allow-simulation` é necessária para aceitá-los; o padrão exige todas as capacidades nativas.

## Regras do primeiro mundo

- Duas conexões, cada uma vinculada ao seu ID de jogador. O pacote de ação não escolhe o dono de outro inventário.
- Somente o estado pessoal do destinatário segue em seu snapshot. Para o outro jogador são públicos nome, pose, equipamento visível e estado vivo/morto.
- Um item de chão tem um ID de mapa estável e global. Coletar remove esse objeto do mundo e cria uma entrada somente na bolsa do vencedor. Descartar volta a criar um objeto compartilhado.
- O primeiro cliente ativo registra objetos do mapa. O ID deve incluir mundo e identidade estável do ator, por exemplo `map:MainField:hash-do-ator`; endereços de memória não são identidades de rede.
- Todo monstro vivo escolhe o jogador elegível mais próximo dentro de `aggro_range`, no mesmo mundo, com pose recente e vida positiva. Se outro ficar estritamente mais perto, troca imediatamente na atualização do servidor. Empates exatos mantêm o alvo atual.
- A simulação da criatura acompanha o jogador escolhido. Cada transferência incrementa `epoch`, invalidando poses e ataques antigos. O cooldown da criatura continua durante a troca.
- Sem alvo elegível, a criatura deixa de ter dono; o futuro backend deve suspender sua IA compartilhada. Mortos, jogadores desconectados e poses com mais de dois segundos não recebem autoridade.
- Atualizações de estado são enviadas a 20 Hz. A lógica não espera uma provocação ou um ataque para selecionar o jogador mais próximo.
- Existe token privado de reconexão, com recuperação em memória por dez minutos após sair. Não é conta, autenticação pública nem persistência após reinício do servidor. Duas instâncias no mesmo PC precisam de tokens e pastas distintos.

## Compilar e testar a base

Na pasta `launcher`, com Go 1.23 ou superior:

```sh
go test -race -count=1 ./internal/...
go vet ./internal/gen1 ./internal/nativebridge ./cmd/hyrulem-gen1-server ./cmd/hyrulem-native-probe
go build -o dist/HyruleM-Gen1-Server.exe ./cmd/hyrulem-gen1-server
go build -o dist/HyruleM-Native-Probe.exe ./cmd/hyrulem-native-probe
```

Em Windows, os executáveis são gerados pelo Actions e publicados como **artefato de desenvolvimento separado**. O servidor Gen1 usa protocolo 2 e, por padrão, porta 5051. O menu visual 0.4.x continua com protocolo 1/porta escolhida pelo usuário e não conecta nesse servidor.

```powershell
.\HyruleM-Gen1-Server.exe --listen 0.0.0.0:5051 --catalog .\catalog.test.json --allow-simulation
```

Esse comando é apenas para desenvolver/testar o protocolo. O catálogo incluído usa nomes sintéticos; não representa as armas/monstros reais. IP de LAN ou Radmin e porta TCP escolhida são aceitos. O servidor não executa BOTW, não contém os arquivos do jogo e ainda não tem cliente nativo jogável.

## Diagnóstico para continuar a integração

1. Abra uma instância pelo multiplayer 0.4.2, escolha Continuar e mantenha a janela renderizando, sem minimizar.
2. Abra `Diagnosticar.cmd` na pasta extraída. Ele executa o diagnóstico do cliente 1 e mantém a janela aberta. Pelo terminal, o equivalente é `HyruleM-Native-Probe.exe --slot 1`.
3. Envie o novo `HyruleM-native-report-DATA-HORA.json` gerado nessa pasta. Para a outra instância use `--slot 2`; `--slot 0` consulta o sandbox normal. O diagnóstico não sobrescreve relatórios existentes.

O diagnóstico `gen1-diag.2` corrige a detecção de porta: a saída do processo multiplayer fica em `logs/multiplayer-processo-N.log` junto da configuração do launcher. No sandbox fica em `logs/sandbox-processo.log`. O log próprio do runtime é somente uma alternativa. `--config` também muda a pasta onde se procuram esses logs; um cliente nunca usa o log do outro. `--port N` continua disponível para uma porta de controle explícita, diferente da porta pública do servidor.

O diagnóstico lê metadados/hash do executável e de `extracted/exefs/main`, e consulta `version`, `help`, `sites`, `mem regions`, `pos`, `camera`, `hm_caps`. Para o módulo 1.9.0 já analisado, com personagem carregado, compara fingerprints de três funções em execução e então lê um conjunto limitado de ponteiros: PlayerInfo, ator local, getter de vida e ActorCreator/contagem de atores. Registra a vida somente se o getter for uma soma simples validável. O relatório não inclui as instruções lidas.

Não faz escrita de memória, não cria itens, não prepara sandbox e não lê saves ou inventário. Um módulo diferente, código divergente ou ponteiro inválido interrompe a inspeção. O relatório pode conter vida do jogador, coordenadas e endereços de memória locais. `functions_verified` é somente a comparação das funções lidas; **não habilita nem comprova criação de atores ou multiplayer nativo**.

É esperado que o Savage Wind oficial responda **unknown command** para `hm_caps`: isso identifica a falta do backend. Um relatório não torna o backend disponível.

O usuário já forneceu o módulo da instalação, e a análise estática confirmou o build 1.9.0 e os pontos descritos em [ANALISE_190.md](ANALISE_190.md). Os arquivos enviados permanecem fora do Git público. A próxima confirmação depende da inspeção no processo em execução e de implementar/validar o backend, sem usar offsets de Cemu/Wii U.

## Limites atuais

O catálogo implementa armas de corpo a corpo e materiais. A fórmula de dano é simplificada, definida no catálogo, em unidades inteiras; não é a fórmula completa de BOTW. Não há armaduras, escudos, arco, flechas, comida, cura, respawn, efeitos elementais, veículos, física compartilhada, eventos, missões nem sincronização geral do mapa. Não há servidor público seguro ou sistema antitrapaça; use sessões privadas de confiança.

Limites da fundação: dois jogadores, 64 monstros, 64 itens de mundo, 128 entradas de inventário por jogador e 512 identidades de registro por sessão. Falta definir streaming completo, Blood Moon e persistência. Bootstrap deve abortar se algum item gerenciado não tiver definição; itens não gerenciados jamais devem ser apagados para caber no catálogo.

[Contrato do backend e pesquisa verificada](NATIVE_BACKEND.md) · [Validação](VALIDACAO.md)
