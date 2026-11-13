# Componentes e fontes

## Launcher próprio

O código do Hyrule Launcher foi criado para este projeto e usa a biblioteca padrão de Go e APIs do Windows. A licença de Go acompanha este projeto em GO_LICENSE.txt.

Não foi copiado código do Milk Bar nem do BreathM para implementar este launcher.

## Recomp incluída no pacote completo

Savage Wind Recomp, v0.5.3-alpha.5:
https://github.com/Revivedx/SavageWindRecomp/releases/tag/v0.5.3-alpha.5

O ZIP oficial é preservado sem alterações em runtime/SavageWind-0.5.3-alpha.5-windows.zip, incluindo seus avisos, licenças e componentes. O launcher confere o hash publicado antes de instalá-lo. O conteúdo é extraído sob a pasta local do aplicativo.

Hash SHA-256:
667c4909a8ed72c002e5aa40a00d38720627eebf48b860beb5cd8805421f1285

O pacote não contém a cópia do jogo, código gerado da cópia de um usuário ou saves.

## Compatibilidade opcional

BotW-Native:
https://github.com/ryderclanky/BotW-Native

Seu código e seu runtime não estão incluídos. O launcher pode iniciar uma instalação existente com os argumentos documentados por esse projeto.


## Referências de formato e integração do sandbox

Os parsers, escritores de formato, a preparação do sandbox e o filtro de campanha foram implementados em Go. Foram consultados os formatos e comportamentos documentados no Savage Wind, as flags nomeadas pelo projeto zeldaret/botw e a biblioteca de formato evfl. Esses projetos não foram incorporados como bibliotecas ao launcher. Os links e a distinção entre o que foi usado e o que continua em estudo estão em ANALISE_SANDBOX.md.

O pacote não inclui saves de exemplo extraídos do jogo nem arquivos de eventos. Os testes usam fixtures sintéticas.

## Verificação de formatos na versão 0.3

Durante o desenvolvimento foram usadas, em pasta de pesquisa separada, as bibliotecas Python evfl e aamp e o leitor BYML do conversor upstream para conferir arquivos originais gerados pelo launcher. Essas bibliotecas de teste não estão incorporadas nem distribuídas com o código do launcher; não são exigidas para usá-lo ou compilá-lo. A documentação de QuestProduct, EventInfo, BFEVFL, AAMP e Demo da comunidade ZeldaMods e o código de engenharia reversa são referenciados em ANALISE_SANDBOX.md.

As substituições de evento e descritor são produzidas pelo código do projeto, localmente. Nenhuma cópia de um evento narrativo original ou de um descritor extraído é incluída na distribuição.
