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
- Emissão compilada do hook no despacho de IA: salto condicional para a continuação, LDR original no caminho normal e CPU preservada. Funções contendo hooks regeneradas/compiladas nos dois modos de registradores.
- Módulo privado Switch 1.9.0: 36 assinaturas próprias únicas, 201 sites resolvidos e despacho Actor -> RootAI executado com proprietário sintético. Não é gameplay.

## Primeiro teste no PC

O candidato inicial foi instalado numa pasta separada, com saves/cache próprios e somente referências de leitura aos recursos originais. A biblioteca privada foi regenerada e carregou os 201 sites. Duas instâncias entraram no mundo com GPU. A captura inicial retornou zero: a leitura da lista real encontrou NPCs de mapa no estado 2, enquanto o filtro aceitava apenas o estado 1 usado pelas marionetes. Depois de um teleporte curto, NPCs passaram ao estado 1 e a mesma varredura capturou 10 atores.

A captura/aplicação agora aceita atores inicializados nos estados 1/2 com modelo presente e referência nativa válida. Inicialização (0), remoção (3), modelo ausente, geração antiga e acesso negado continuam excluídos. A regressão sintética inclui NPC no estado 2. A captura AS de NPCs também deixa de depender da existência do PlayerInfo, para conservar o pedido inicial antes do Link; esse caso tem fixture própria. No segundo candidato houve 26/26 correspondências e nenhuma ausência na área de teste, mas a comparação independente das matrizes na memória do cliente detectou divergência máxima de 21,60 m em NPCs. Portanto esse teste reprovou, apesar dos contadores de aplicação. Não é um resultado de mundo sincronizado validado.

O hook passou da implementação base de RootAI para o despacho virtual comum do Actor, antes da chamada, cobrindo implementações substituídas. O retorno condicional agora salta apenas a chamada de IA e continua o restante da atualização do Actor; não retorna indevidamente da função chamadora. A captura inicial de AS antes do PlayerInfo é independente desta correção. A nova revisão precisa repetir a conferência de posições e rendering; o acesso remoto terminou por timeout antes disso. A candidata permanece sem publicação.

## Limites e próximo teste

Não estabelece ainda mundo integralmente idêntico: atores ausentes não são criados, extras não são ocultados, atores dinâmicos sem identidade de mapa não entram neste fluxo, e objetos fora do raio/orçamento permanecem locais. IA sincronizada aqui significa suspender a decisão local e reproduzir os pedidos recebidos; não há combate remoto nem seleção do Link remoto como alvo. Não transmite a fase exata de todas as animações, física completa, agenda/diálogo, flags ou persistência.

O próximo teste deve usar cópias dos saves em dois processos Windows com GPU: observar NPCs/monstros/props correspondentes, confirmar contadores de captura/correspondência/aplicação e IA suspensa apenas no seguidor, provocar movimento no host e verificar perda/retorno do host e carregamento. Contador crescente não substitui observação do movimento desenhado. Até esse teste ser concluído, rendering desta camada permanece não verificado.

ABI 11, protocolo 11, gamelib ABI 4. Há um novo hook: regeneração privada obrigatória. Nada do módulo, recursos, saves, código de jogo gerado ou disassembly integra a publicação.

## Retomada: teste da revisão com despacho virtual

A revisão 462f5178 foi instalada e a biblioteca privada regenerada no PC. Foram novamente encontrados 26/26 atores, sem identidades duplicadas. Leituras independentes observaram até 25,16 m de diferença no ator e 16,41 m na raiz do modelo. Outros instantes ficaram abaixo de 0,13 m: a aplicação alternava com transformações locais entre fases, sem estabilidade suficiente para aprovação.

A nova interpolação continua da última matriz aplicada pela rede, armazenada no próprio binding, em vez de recomeçar da matriz que a simulação local sobrescreve. A regressão injeta essa sobrescrita entre pacotes e exige continuidade.

Os pedidos AS já chegavam com nomes como Walk, Run e Face_C_Default, mas o receptor interpretava o registrador de retorno do wrapper nativo como sucesso. Esse wrapper não oferece um retorno booleano confiável; a nova confirmação usa o hook de entrada do pedido, após lookup de recurso, e não reinicia repetidamente uma animação já aceita. A captura normaliza índices de grupo/slot como o motor antes de limitar os dois canais transmitidos. Testes incluem índices padrão, retorno zero com pedido aceito e retorno não zero sem pedido.

Estas correções ainda aguardam novo teste GPU. Reconhecimento, pedido aceito e modelo desenhado continuam evidências diferentes.

## GPU: revisão 0a0b644d e experimento seguinte

A revisão 0a0b644d passou no CI Linux/Windows e na construção/instalação do runtime. No PC com duas instâncias, a continuação da trajetória de rede manteve as raízes dos modelos próximas das capturas do host, e o hook confirmou pedidos AS (41 na rua de Hateno; 116 no acampamento). O timeout devolveu world_role a zero. Esses números não comprovam física nem todos os ossos desenhados.

O primeiro teste no acampamento foi inválido para correspondência: o segundo teleporte não terminou e o cliente permaneceu em Hateno. Os relatórios desse teste foram conservados. O teste seguinte exigiu estado done do teleporte e conferiu as coordenadas reais nos dois processos antes de abrir a sessão. Nesse teste, os últimos quadros encontraram 23/23 e 20/20 atores; quatro leituras independentes compararam 23/23, sem duplicatas, com erro máximo de raiz de modelo de 0,323 m. O máximo de posição interna chegou a 58,92 m em NPCs inativos: simulation_ok permaneceu false e ok global continuou false.

A inspeção adicional leu a paleta de ossos usada no desenho. Algumas raízes de Bokoblins estavam próximas do ator e outras ainda divergiam, apesar da raiz do ModelGroup correta. As imagens não foram aceitas como prova completa: vegetação/ângulo dificultaram observar todos os inimigos. Portanto visual_ok do instrumento antigo significa somente raiz do ModelGroup; não certifica a paleta nem o rendering.

O experimento seguinte ativa a notificação nativa de física na segunda chamada de hm_set_mtx (a atualização imediata), mantendo a primeira chamada pendente e todos os controles de referências/autoridade. A análise privada anterior mostrou que esse argumento aciona os caminhos adicionais do PhysicsSystem; não altera estado/sleep flags por escrita bruta. O teste sintético exige o contrato do argumento somente nas transformações do seguidor e mantém CPU/isolamento. Ainda é hipótese a validar com GPU e paleta, não correção comprovada. Nenhuma preview.15 foi publicada.
