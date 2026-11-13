# Módulo recebido: BOTW Switch 1.9.0

**Análise estática concluída e objetos confirmados pelo relatório do processo; criação do Link remoto ainda não validada em gameplay.** Os resultados sustentam o adaptador experimental, sem constituir uma Gen1 jogável completa.

O RAR fornecido contém `main`, `main.npdm`, `rtld`, `sdk` e `subsdk0`. Foi extraído em uma área privada de pesquisa, fora da árvore versionada. Nenhum desses arquivos, imagem descomprimida ou disassembly completo entra no repositório ou no pacote.

## Identidade e método

- `main`: 18.782.505 bytes; SHA256 `f6f0b20bb1f2b67d164c24871cd473147eef1d285f12e7bd492e3126c6022e27`, igual ao relatório do usuário.
- Build ID sem padding: `cd57b23fa4bbad65803d9788c01821ee`. A tabela do Savage Wind identifica esse build como 1.9.0.
- Segmento de código: 22.682.724 bytes após descompressão. Base de análise do runtime: `0x7000000000`.
- Resolução dos pontos existentes pelo `tools/game_sites.py` do Savage Wind fixado, com resultados compatíveis com a tabela conhecida. Os novos nomes foram inferidos pelo fluxo de chamadas e comparação com a decompilação 1.5.0; os endereços dessa versão antiga não foram reutilizados.
- As fronteiras das funções foram obtidas pelo unwind do próprio módulo. O mapa `launcher/internal/nativebridge/native_190_sites.json` registra endereços, tamanhos e hashes de 14 funções, mais cinco relações de chamada. Algumas funções pequenas têm assinaturas duplicadas; essas dependem da identidade completa do módulo e do endereço, sem seleção por primeira ocorrência.

## Pontos relevantes identificados

| Rotina ou objeto | Endereço na análise | O que foi observado |
|---|---|---|
| Criação síncrona de ator, com verificação do tipo retornado | `0x7001535900` | Chama o helper de perfil/criação; verifica o tipo e agenda remoção se incompatível |
| Helper de perfil e classe | `0x7001535954` | Resolve o perfil, classe, estado de bloqueio e chama a criação interna |
| Criação interna | `0x7001535798` | Monta os dados da criação e chega ao gerenciador de processos |
| Pedido assíncrono de criação | `0x7001535b10` | Resolve perfil/classe e encaminha heap, handle, parâmetros, objeto do mapa e fila |
| Pedido interno | `0x7001535a08` | Monta a tarefa e chama o gerenciador de processos; verifica bloqueio de criação |
| Registro do jogador | `0x7000cc6f54` | Retorna sem registrar se PlayerInfo já tem um ator; caso vazio, grava o ator e adquire seu link |
| Obtenção do jogador | `0x7000cc6fd0` | Lê PlayerInfo + `0x60`, verificando acesso pelo gerenciador |
| Leitura de vida do jogador | `0x7000cc7184` | Chama o getter virtual do ator pelo slot `0x2b8` e lê o inteiro retornado |
| Escrita de vida do jogador | `0x7000cc7150` | Usa o mesmo getter e grava o valor; não é invocada pelo diagnóstico |
| Global PlayerInfo | `0x7001e13ed0` | Ponteiro para a instância do jogador |
| Global ActorCreator | `0x7001e95430` | Referência confirmada por chamadores da criação e pela entrada GOT correspondente |
| Global BaseProcMgr | `0x7001e937f8` | Referência usada por criação e validação de acesso a atores |

A existência do bloqueio de registro do jogador é promissora para um proxy, mas não prova que duas instâncias da classe Player possam funcionar de forma independente. Entrada, câmera, inventário, equipamentos, efeitos e outras referências globais ainda exigem investigação. Nenhuma chamada de criação foi executada aqui.

## O que o relatório revelou e o que foi corrigido

O relatório enviado contém `control_port=0` e `queries=null`: nenhuma consulta ao runtime chegou a ocorrer. A versão inicial procurava somente `DataDir/.../savagewind.log`, enquanto `Session.Start` grava stdout/stderr em `AppDir/logs/multiplayer-processo-N.log`. Isso era uma falha do diagnóstico, não uma incompatibilidade provada do jogo.

`gen1-diag.2` corrige o caminho, respeita o cliente selecionado e a pasta da configuração. `Diagnosticar.cmd` permite executar com dois cliques, e cada relatório ganha data/hora.

Com o mundo carregado e o módulo exato, o diagnóstico compara hashes de três funções **lidas do processo** antes de seguir ponteiros. Só então identifica PlayerInfo, ator local, getter de vida e ActorCreator. O getter de vida só é decodificado quando consiste em uma ou duas somas de endereço seguidas de retorno; nenhuma função é executada. O relatório inclui valores interpretados, sem copiar código do jogo. Código divergente, ponteiro nulo ou módulo desconhecido encerra/omite essa inspeção.

## Reproduzir a validação estática

Ambiente de desenvolvimento com Python e `lz4`, mantendo o módulo fora do Git:

```sh
python launcher/tools/verify_native_sites.py /caminho/privado/exefs/main
```

O verificador recusa outro SHA256, confere funções/assinaturas e destinos de chamadas e devolve `native_backend_ready=false`. Os testes Go usam fixtures próprias e verificam a recusa de código divergente, ponteiros nulos e getters com chamadas ou registradores diferentes.

## Próximo passo de integração

O relatório posterior confirmou os hashes das funções no processo, jogador local, vida e 620 atores registrados. O adaptador em `native/` já implementa fila na thread do jogo, criação assíncrona, referências, transformação/animação e remoção. Doze assinaturas adicionais foram verificadas como únicas no módulo fornecido. O gerador privado de recursos foi conferido também por um parser independente: os 7.944 registros originais de atores foram preservados e um novo registro foi adicionado.

O teste do usuário revelou presença invisível. A inspeção posterior identificou o controlador de dano como receptor do callback, com o ator dono em `this + 0x18`. Preview 2 corrige essa comparação e adiciona convergência de vida dos monstros por HashId, tipo e posição original, usando protocolo 3. Coleta, PvP, colisão e IA compartilhada continuam pendentes; não são declaradas as capacidades completas Gen1. A correção ainda precisa de confirmação visual com dois jogos.

Referências fixadas: [Savage Wind](https://github.com/Revivedx/SavageWindRecomp/tree/740b18025fe11ba03c117dbb56668db304b5043e) e [zeldaret/botw](https://github.com/zeldaret/botw/tree/a0fae63049c578629fdcc8173d8ef888dd4d9e5a), especialmente ActorCreator, PlayerInfo, InstParamPack e a tabela de funções. A versão 1.5.0 é referência de interpretação; os fingerprints foram obtidos e verificados no módulo 1.9.0 fornecido.
