# Auditoria - 0.5.0-preview.15

## Escopo

Primeira autoridade de mundo sobre atores de mapa já carregados: monstros comuns, NPCs, lista explícita de objetos móveis e armas soltas. Até 64 identidades em 180 m do anfitrião. Hash de mapa, nome e posição original unem os atores entre processos; IDs de rede não são ponteiros.

O bridge publica a captura nativa por comando separado a cada 100 ms. O servidor aceita somente o anfitrião eleito e a época atual, descarta sequências repetidas e capturas anteriores a um teleporte. O quadro binário HMW1 tem cabeçalho de 8 bytes e registros de 300 bytes; o comando máximo testado tem 25.707 bytes. A sessão de mundo existe mesmo com o experimento de itens desligado.

O runtime aplica matrizes e até dois pedidos AS aos atores reais correspondentes, após os jobs, usando referências retidas e acesso local temporário. Faz interpolação de translação e rotação por quaternion normalizado; saltos maiores que 20 m são instantâneos. A função RootAI original só é ignorada para atores seguidores com geração correta e escrita recente. Heartbeat vencido, mudança de papel/época/sessão e término liberam as referências.

Não há spawn/delete arbitrário neste caminho. Armas equipadas são excluídas. Os fluxos anteriores de dano e coleta/descarte não foram substituídos. Pausa local, carregamento ou ausência de ator não são morte. Um ator com identidade alterada durante a aquisição da referência é solto antes da captura seguinte, evitando um ID zero persistente.

## Verificações executadas

- Go/race: transporte TCP real com dois clientes, publicação restrita ao host, troca de host, entrada tardia, replay, época antiga e quadros completos de 64 atores pelo ControlStream usado pelo launcher.
- C sintético: captura host, correspondência por mapa, escrita de transformações nativas, NPC com um canal AS, ausência sem criação/destruição, arma equipada excluída, geração antiga, acesso negado e liberação, timeout, isolamento do jogador/inventário e identidade alterada durante resolução.
- Interpolação de rotação exercitada a 30/60/90/120 atualizações por segundo, incluindo meia-volta sem matriz degenerada.
- Emissão compilada do hook RootAI: retorno condicional, prólogo original e CPU preservada. Funções contendo hooks regeneradas/compiladas nos dois modos de registradores.
- Módulo privado Switch 1.9.0: 36 assinaturas próprias únicas, 201 sites resolvidos e despacho Actor -> RootAI executado com proprietário sintético. Não é gameplay.

## Limites e próximo teste

Não estabelece ainda mundo integralmente idêntico: atores ausentes não são criados, extras não são ocultados, atores dinâmicos sem identidade de mapa não entram neste fluxo, e objetos fora do raio/orçamento permanecem locais. IA sincronizada aqui significa suspender a decisão local e reproduzir os pedidos recebidos; não há combate remoto nem seleção do Link remoto como alvo. Não transmite a fase exata de todas as animações, física completa, agenda/diálogo, flags ou persistência.

O próximo teste deve usar cópias dos saves em dois processos Windows com GPU: observar NPCs/monstros/props correspondentes, confirmar contadores de captura/correspondência/aplicação e IA suspensa apenas no seguidor, provocar movimento no host e verificar perda/retorno do host e carregamento. Contador crescente não substitui observação do movimento desenhado. Até esse teste ser concluído, rendering desta camada permanece não verificado.

ABI 11, protocolo 11, gamelib ABI 4. Há um novo hook: regeneração privada obrigatória. Nada do módulo, recursos, saves, código de jogo gerado ou disassembly integra a publicação.
