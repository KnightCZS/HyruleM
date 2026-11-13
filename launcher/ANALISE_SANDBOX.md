# Análise técnica — filtro de campanha 0.3.0

## Arquitetura implementada

A confirmação do usuário na versão 0.2 foi o início em Hateno, sem torres nem runas. A versão 0.3 mantém esse sandbox e acrescenta uma camada de recursos para filtrar a campanha. Não depende de recompilar a DLL do jogo nem modifica o dump original.

O launcher lê os pacotes da instalação, gera um overlay separado e define `SAVAGEWIND_ROMFS_OVERLAY` apenas ao iniciar o sandbox. A campanha normal é iniciada sem esse overlay. O save continua separado por `SAVAGEWIND_SAVE_DIR`.

### Fluxo de preparação

1. Ler a política e identificar os arquivos da instalação e de DLCs reconhecidos.
2. Ler QuestProduct e EventInfo para obter as categorias e nomes reais da versão local.
3. Reunir os pontos de entrada dos fluxos bloqueados, incluindo auxiliares internos e cópias presentes em outros pacotes.
4. Filtrar as missões Main, preservando Mini e, por padrão, Sub.
5. Gerar substituições narrativas com uma ação EventSystemActor/Demo_WaitFrame de um quadro, seguida de término normal.
6. Ajustar o EventInfo para carregar esses fluxos em lugar de timelines ou filmes e gerar descritores próprios sem reposicionamento ou câmera narrativa.
7. Retirar EventTags de mapa que chamem os fluxos bloqueados e limpar ligações para essas tags. Outros atores permanecem.
8. Atualizar a RSTB para os recursos gerados e publicar a camada completa após conferir a estabilidade dos arquivos de origem.

O uso de um evento válido, com um ator e ação vinculados, é intencional. A integração do jogo verifica bindings; um fluxo completamente vazio pode ser rejeitado antes de executar. A substituição mantém as entradas que outros fluxos esperam chamar.

O programa preserva os demais arquivos dos pacotes e os dados das missões secundárias. Não marca a campanha como concluída, não remove textos de diálogos comuns e não substitui requisitos de habilidades ou itens das secundárias por permissões de servidor.

## Classificação

As categorias do próprio QuestProduct orientam a filtragem: Main, Mini e Sub. As famílias narrativas de demos são tratadas com as exceções documentadas. Não é aplicado um bloqueio geral de tudo que começa com Demo: baús, cozinha, viagem, serviços e santuários também usam esse sistema.

Foi examinada uma referência pública com 5.608 registros de EventInfo para conferir o formato e a distinção entre nomes de fluxo e entradas. Esse material é uma referência de engenharia reversa; os arquivos aplicados ao jogo são gerados a partir da instalação local, não desse catálogo público antigo.

A política permite manter um fluxo específico ou bloquear outro. Para manter estritamente as secundárias Mini, `KeepShrineQuests` pode ser definido como false. Os resultados concretos ficam no relatório gerado na máquina do jogador.

## Preservação e cache

A camada é preparada em uma pasta temporária. Falhas não substituem o filtro anterior nem iniciam o jogo dizendo que uma geração incompleta foi aplicada. A versão anterior da camada é mantida separadamente após a publicação.

O cache considera a política, a lista de candidatos, tamanhos e datas dos arquivos, além de SHA-256 dos pacotes de inicialização e tabelas críticas. A existência, tamanho e data dos arquivos gerados também são conferidos. Atualizar filtro força nova geração. Os saves não são reiniciados.

O runtime pode reaplicar opções de `launch-flags.txt` depois da criação do processo. Por isso o launcher detecta opções que substituiriam a pasta de saves, o overlay ou os parâmetros de controle/teste da sessão. Ele não regrava as opções gráficas do jogador.

## Fontes primárias

O runtime estudado é Savage Wind `v0.5.3-alpha.5`, commit `740b18025fe11ba03c117dbb56668db304b5043e`.

| Tema | Referência |
|---|---|
| Overlay, pacotes e RSTB no runtime | [fs.c](https://github.com/Revivedx/SavageWindRecomp/blob/v0.5.3-alpha.5/runtime/services/fs.c) |
| Organização do RomFS extraído | [romfs_image.c](https://github.com/Revivedx/SavageWindRecomp/blob/v0.5.3-alpha.5/runtime/romfs_image.c) |
| Saves separados | [save-profiles.md](https://github.com/Revivedx/SavageWindRecomp/blob/v0.5.3-alpha.5/docs/save-profiles.md) |
| Opções reaplicadas no início | [launch_flags.h](https://github.com/Revivedx/SavageWindRecomp/blob/v0.5.3-alpha.5/runtime/launch_flags.h) |
| Categorias e estrutura das missões | [QuestProduct](https://zeldamods.org/wiki/QuestProduct.bquestpack) |
| Metadados de eventos | [EventInfo](https://zeldamods.org/wiki/EventInfo.product.sbyml) |
| Famílias e exceções das cenas | [Demo](https://zeldamods.org/wiki/Demo) |
| Estruturas dos fluxos | [BFEVFL](https://zeldamods.org/wiki/BFEVFL) e [evfl](https://github.com/zeldamods/evfl) |
| Parâmetros dos descritores | [AAMP](https://zeldamods.org/wiki/AAMP) e [resResourceDemo.cpp](https://github.com/zeldaret/botw/blob/a0fae63049c578629fdcc8173d8ef888dd4d9e5a/src/KingSystem/Resource/Event/resResourceDemo.cpp) |
| Carregamento e bindings | [evtResourceFlowchart.cpp](https://github.com/zeldaret/botw/blob/a0fae63049c578629fdcc8173d8ef888dd4d9e5a/src/KingSystem/Event/evtResourceFlowchart.cpp) |
| Agendas e eventos de etapas | [qstStep.cpp](https://github.com/zeldaret/botw/blob/a0fae63049c578629fdcc8173d8ef888dd4d9e5a/src/KingSystem/Quest/qstStep.cpp) |
| Referência pública de parâmetros | [leoetlino/botw](https://github.com/leoetlino/botw), especialmente Event/EventInfo.yml |

Os leitores, escritores e a integração do launcher foram implementados em Go, sem incorporar essas bibliotecas Python como dependências do executável. Elas foram usadas separadamente para verificar recursos gerados durante o desenvolvimento. Nenhum dump, save ou arquivo de evento do jogador é distribuído no pacote.

## Limites da validação

Passaram os testes de formatos, preservação, geração, cache, política, falhas e ligação do overlay ao sandbox. Os recursos gerados também foram lidos por parsers independentes de BYML, BFEVFL e AAMP.

Ainda não foi executada uma sessão real de BOTW com o filtro 0.3 neste ambiente. Isso deixa pendentes o comportamento de NPCs, chefes, diário, carregamentos e gatilhos em todas as regiões. O relatório mantém `VerifiedInGame: false`; não existe uma confirmação automática de que toda combinação de missão e cena foi validada.

A camada não é multiplayer e não implementa autoridade de servidor, economia, persistência RP ou permissões de habilidades. Ela prepara a base local para esses sistemas.
