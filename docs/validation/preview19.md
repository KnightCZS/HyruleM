# Validação da prévia 19

Executada em 21 de novembro de 2026, no Windows com GPU e dois jogos Switch 1.9.0. Ambos foram abertos pelo launcher e conectados a um processo independente do servidor dedicado.

- Commit do pacote: `ab14775bdb58de23c585ef16498da5eed5de1dee`.
- Protocolo 15, referências 16, ABI nativa 13.
- ZIP Windows: 25.656.467 bytes; SHA-256 `a3a0bd9fd69d0cd51c0e59f1394c8d22cd2f0ba40f9144456a5fe5c3d001e2d5`.
- [Build nativo](https://github.com/KnightCZS/HyruleM/actions/runs/44953391156) e [build geral](https://github.com/KnightCZS/HyruleM/actions/runs/44953391001) passaram.

## Observações no jogo

| Caso | Evidência |
| --- | --- |
| NPC ausente | `Npc_HatenoVillage022`, hash 763536536, removido somente do receptor por comando de diagnóstico. O adaptador criou um ator nativo sem objeto de mapa, associou a identidade canônica e voltou a encontrá-lo no quadro remoto. A captura de tela também mostra o NPC. |
| Inimigo ausente | Mesmo procedimento para `Enemy_Bokoblin_Junior`, hash 277007734. A lista nativa confirmou a troca do original por uma reconstrução sem hash de mapa; contador de criação 1, falhas 0. Esta evidência é da lista nativa, não de uma comparação visual quadro a quadro. |
| Morte compartilhada | Vida 13 → 0 nos dois fluxos; nenhum ator ativo com o hash original ficou na lista nativa dos dois jogos após o combate. |
| Árvores | Pinheiro de Rito e árvore larga de outra região viraram tocos nos dois jogos. Cada corte foi executado por um cliente diferente. |
| Coleta | Maçã global 7 terminou coletada; exatamente um recibo nativo de crédito, no cliente 1. |
| Distância | Separação superior a 2 km por 22 segundos e reunião sem encerrar a integração. |
| Saída e retorno | O primeiro cliente saiu, o segundo continuou e o primeiro reconectou. |
| Reinício do dedicado | Mesmo UUID de mundo e maçã ainda coletada após encerrar e reabrir o processo servidor. Registro final: 65 entidades, 7 itens e 5 identidades de vida. |
| Transporte | 16.191 snapshots observados, 13.260 com referência ao quadro anterior (81,9%); 22.222.320 bytes de blobs de atores repetidos suprimidos. Não é uma medição de FPS ou da banda total economizada. |

Passaram também Go com detector de races, C com memória sintética, emissão de hooks, builds Windows/Linux e verificações do instalador. Os novos testes cobrem exclusão das reconstruções da eleição de simulador, estado sem cliente simulador, vida inicial canônica, liberação de cópias mortas, preferência pelo ator original e repetição da limpeza quando o acesso nativo é negado.

## Cobertura que continua faltando

As reconstruções acompanham o estado recebido; não assumem a simulação nem reproduzem todos os parâmetros de criação de mapa, equipamentos de inimigos ou conversas de NPCs. Sem cliente com um ator original elegível, o servidor conserva o último estado. Ainda não há uma instância completa do motor de BOTW executando no dedicado.

A sincronização total permanece em desenvolvimento: espólios e troncos dinâmicos, divergência de tipo/escala entre saves, convergência física geral, projéteis, eventos e interações arbitrárias precisam de implementação e validação. Os 44 recursos de toco foram catalogados, mas somente duas variedades receberam o teste de corte acima. Os inventários são privados dos clientes; a coleta dos itens integrados usa reserva exclusiva no servidor. A prévia continua limitada a dois jogadores, 64 atores próximos e 32 itens na janela nativa por cliente. Esta execução usou dois jogos no mesmo PC.
