# Servidor dedicado do HyruleM

O servidor é um executável independente para Windows x64 ou Linux x64. Guarda o mundo, decide qual cliente pode simular cada entidade e arbitra as coletas. O jogo recompilado roda nos clientes atribuídos pelo servidor.

## Executar

Extraia o pacote do servidor e escolha a porta.

Linux

```sh
./HyruleM-Server --listen 0.0.0.0:5050 --world ./world/world.json
```

Windows

```powershell
.\HyruleM-Server.exe --listen 0.0.0.0:5050 --world .\world\world.json
```

Os jogadores informam o IP ou DNS dessa máquina e a mesma porta no launcher. A porta usa TCP. O endereço pode pertencer à rede local, a uma VPN ou à interface pública configurada pelo administrador.

Ctrl+C ou SIGTERM encerra o servidor e grava o estado. O arquivo do mundo e sua pasta devem acompanhar uma migração de máquina. O processo mantém um bloqueio do arquivo para impedir duas instâncias de escreverem no mesmo mundo.

## Autoridade e carregamento

Cada entidade de mapa tem uma identidade estável e um cliente responsável escolhido pelo servidor entre os que a têm carregada. A escolha usa proximidade e disponibilidade. Uma revisão de autoridade invalida publicações antigas durante a transferência. O receptor aplica o estado na thread do jogo após o término das tarefas dos atores.

Uma entidade carregada novamente recebe seu estado conhecido antes de assumir a simulação. O servidor mantém registros de entidades, vida e itens entre regiões e reinícios. Com nenhum cliente perto de uma entidade, o registro conserva o último estado. A execução da IA nativa depende de um cliente com essa entidade carregada.

Os setores do mundo aberto pertencem ao mesmo cenário. Interiores usam o tipo e o nome do mapa, identificados por assinaturas da versão Switch 1.9.0. Jogadores e objetos de santuários distintos têm identidades separadas mesmo quando suas coordenadas coincidem.

O servidor controla o horário e o clima. Um dia dura 24 minutos por padrão. Altere esse valor com `--day-length 48m`. Pausar um cliente preserva o relógio do servidor.

## Estado e itens

A gravação usa um arquivo temporário sincronizado e substituição do arquivo anterior. Uma reserva de coleta é gravada antes de autorizar a escrita no inventário. A confirmação final também é gravada. Um arquivo corrompido interrompe a abertura do servidor e é preservado para diagnóstico. Uma falha de gravação durante a sessão encerra as conexões e faz o processo terminar com erro.

Uma reserva interrompida durante uma escrita de inventário fica com resultado incerto. Ela permanece bloqueada, pois o servidor precisa reconciliar se o inventário já recebeu o item antes de disponibilizá-lo novamente.

O registro aceita até 32.768 identidades de itens. Cada cliente acompanha até 32 próximos por vez. Os slots locais são reutilizados após a confirmação de limpeza pelo jogo. Materiais comuns de mapa e famílias de armas usam a reserva exclusiva. A coleta do mesmo objeto só pode conceder um inventário.

## Cobertura atual

Esta prévia aceita dois jogadores. A captura nativa acompanha até 64 atores por cliente em 180 m. O transporte funciona em processos e máquinas diferentes.

Continuam pendentes a criação geral de atores ausentes, o ciclo dos espólios dinâmicos, troncos e madeira, os eventos gerais de missões, projéteis e a validação completa da física e do combate. A identificação de interiores está integrada ao protocolo. O gameplay em todos os interiores ainda exige validação.
