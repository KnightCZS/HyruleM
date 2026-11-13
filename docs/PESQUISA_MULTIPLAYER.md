# Pesquisa para o primeiro multiplayer

Pesquisa de 13/11/2026. Referência do runtime: Savage Wind `v0.5.3-alpha.5`, commit `740b18025fe11ba03c117dbb56668db304b5043e`. A implementação deste repositório foi escrita de forma independente; os projetos abaixo são referências técnicas, não componentes incorporados.

## Três caminhos

| Caminho | O que permite | Custo e limite |
| --- | --- | --- |
| Adaptar Milk Bar/Cemu | Usar atores remotos e infraestrutura conhecida no Wii U | Endereços PowerPC, DLL injetada e pacotes de atores próprios do Cemu não correspondem ao runtime Switch recompilado |
| Ator nativo no Savage Wind | Modelo, animação, oclusão e integração correta com o jogo | Exige resolver criação/destruição de ator e transformações no código recompilado; não existe comando público pronto para criar um segundo Link |
| Telemetria + avatar externo | Testar conexão, duas instâncias, presença e movimento sem recompilar o jogo | Imagem provisória, sem colisão/oclusão; não é ator nativo |

Escolha da 0.4: terceiro caminho, expondo sua limitação na interface. A divisão entre protocolo, ponte de telemetria e renderizador permite substituir o desenho externo por um ator nativo sem reescrever o servidor.

## Fontes primárias consultadas

- [MilkBarLauncher](https://github.com/MilkBarModding/MilkBarLauncher): servidor C#, cliente C++ injetado no Cemu, frontend WPF e comunicação entre componentes. Consultados README e `DLL/InjectDLL/Player.cpp` do commit `22d5184c65677d09ec3de78ab596e55bd765aaab`. O fluxo mantém representações remotas e atualiza suas transformações. Não foram copiados código nem BNPs; as condições de redistribuição do projeto também precisam ser consideradas antes de qualquer adaptação futura.
- [SpawnActors](https://github.com/S41L0R/SpawnActors): referência de criação de atores no BOTW Wii U/Cemu; não fornece diretamente endereços/chamadas válidas no Savage Wind.
- [BreathM](https://github.com/msork/BreathM): o README consultado apresenta um projeto inicial; não foi tratado como uma implementação de sincronização de gameplay pronta para mesclar.
- [Savage Wind: controle](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/docs/CONTROL.md): comandos locais `pos`, `camera`, `goto`, `teleport status`, `get`, `cell` e `mem read`; respostas JSON por linhas.
- [Leitura da câmera](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/control_game.c): `camera` usa a matriz do frame quando disponível, ou a câmera orbital; fornece posição e direção, mas não a projeção completa.
- [Células da câmera](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/control_cells.c): cadeias conhecidas de eye/target da versão 1.9.0. A interpretação dos campos adjacentes de FOV/up feita neste protótipo é uma hipótese validada estruturalmente, ainda sem validação visual no jogo real.
- [Projeção do renderer](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/nvn/output_scale.c): o FOV percentual escala o meio ângulo; a janela também afeta o aspecto. A calibração externa não replica transições especiais de câmera.
- [Pastas do runtime](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/paths.c), [settings](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/settings.c) e [gamelib](https://github.com/Revivedx/SavageWindRecomp/blob/740b18025fe11ba03c117dbb56668db304b5043e/runtime/gamelib.c): dados por `SAVAGEWIND_DATA_ROOT`, configurações por `SAVAGEWIND_SETTINGS`, DLL do jogo por `SAVAGEWIND_GAME_LIB`. Usados para separar os dois clientes.

## Decisões de rede

TCP com JSON por linha para reduzir o custo de depuração do primeiro teste de duas pessoas. É suficiente para duas poses a 10–20 Hz em LAN; TCP pode sofrer atraso por retransmissão em redes ruins. O protocolo versão 1 possui hello/welcome, pose, snapshot, rename, ping e leave. IDs e slots vêm do servidor. Limite de duas conexões registradas e oito sockets pendentes, nomes limitados, valores finitos e pacotes de até 16 KiB; deadlines impedem conexão travada de bloquear indefinidamente o servidor.

O servidor é retransmissor de poses, não simulação autoritativa do mundo. Não verifica terreno nem combate. Nenhum comando recebido da rede é repassado arbitrariamente ao controle do jogo. A ponte usa apenas comandos construídos localmente e um ponto de spawn validado recebido na conexão. O controle de memória permanece em localhost; o servidor só recebe nome, posição, velocidade, orientação aproximada e fase de caminhada.

Snapshots são interpolados com 100 ms de atraso; saltos acima de 20 m não são interpolados. Câmera velha, desconexão e jogador não pronto ocultam a representação remota. Uma futura integração poderá adicionar sequências por jogador, timestamps e UDP/QUIC, mas isso não resolve a parte de criar ator no jogo.

## Próxima integração nativa

1. Adicionar ao runtime um comando de projeção/câmera completo e identificação de MainField/interior, evitando hipóteses de calibração.
2. Identificar a função de criação/destruição de um ator remoto em BOTW Switch 1.9.0 e seus requisitos de recursos/modelo. Validar no thread correto do jogo.
3. Criar uma representação sem IA local, sem registrar como o jogador principal e sem gravar no save.
4. Aplicar posição/rotação/animação por ID, destruir ao desconectar e no unload/reboot do mundo.
5. Substituir o renderizador externo; manter servidor e menu. Confirmar nas duas perspectivas do jogo antes de dizer que existe um segundo Link nativo.

Nenhum offset ou assinatura de spawn foi inventado nesta versão.
