# HyruleM 0.5.0-preview.18.1 — multiplayer experimental

O servidor dedicado mantém o registro persistente do mundo e decide o responsável por cada entidade carregada. Ambos os clientes capturam seus arredores. Os demais recebem o estado correspondente e suspendem a decisão local de IA enquanto aplicam dados recentes. O modo pode ser hospedado pelo launcher ou pelo executável independente. Veja [como executar o servidor](../docs/DEDICATED_SERVER.md).

A revisão c64d0fc7 foi testada em dois jogos Windows com GPU. Em Hateno, encontrou 28/28 atores e passou nas sete comparações das raízes dos esqueletos ativos, com diferença máxima observada de 1,36 m em relação ao anfitrião. As imagens também foram conferidas. No acampamento de Bokoblins, as últimas seis amostras ficaram abaixo de 1 m; a primeira chegou a 2,01 m. A correção mantém a trajetória de rede, confirma os pedidos AS e reposiciona a paleta final de ossos, que antes podia permanecer longe da raiz do modelo.

**Esta é uma etapa visual, com atraso e animações ainda aproximadas.** O teste geral de física continua reprovado: posições internas de NPCs inativos divergiram até 17,63 m em Hateno, embora suas raízes de modelo acompanhassem o anfitrião. Não anuncia colisões, combate ou simulação integral compartilhados. O último relato de efeitos de espada/impacto e algumas roupas continua pendente. Resultados e cobertura estão na [auditoria](https://github.com/KnightCZS/HyruleM/blob/v0.5.0-preview.15/docs/AUDITORIA_0.5.0-preview.15.md).

## Atualizar e usar

1. Feche ambos os jogos e o launcher. Extraia o ZIP inteiro, incluindo `runtime-update`.
2. Execute **Iniciar-Nativo.cmd**, que instala o runtime com backup.
3. O launcher confere e, quando necessário, recompila a biblioteca privada compartilhada antes de abrir os clientes. Aguarde o término. A prévia 18 exige ABI 12 e acrescenta as raízes de identidade do cenário. Atualize os dois clientes e o servidor.
4. Conecte pelo IP ou DNS do anfitrião e pela porta TCP escolhida. Clientes e servidores das prévias 15 e 16 podem jogar juntos. Com cliente receptor e servidor 16, o primeiro quadro dos atores vai completo e repetições usam uma referência curta. Cada nova captura continua levando a lista completa. A idade da captura continua contando durante as repetições.
5. Hospede e conecte pelo IP/porta escolhidos. Na primeira abertura desta versão, **Itens do mundo: ligados** é ativado no painel de hospedagem. Depois disso a escolha é preservada; pode desligá-la antes de hospedar. O servidor separado usa `--shared-items=true` por padrão, ou `--shared-items=false` para desligar.
6. Para hospedar uma sessão privada local, escolha **Somente este PC**. Para testar no mesmo PC, expanda **Testar dois jogos neste PC**. Cada instância tem save, cache, configurações e porta de controle próprios.

## Atraso na visão do outro jogador

O servidor passa de snapshots padrão a cada 50 ms para 20 ms. O cliente troca a espera fixa de 100 ms por uma reserva adaptativa entre 20 e 100 ms: num fluxo sintético estável de 20 ms ela é de 25 ms. A reserva aumenta com jitter; não prevê ossos ou armas separadamente. Esses números descrevem o buffer, não uma medição de ping ou do atraso total no PC do jogador.

O servidor não devolve ao próprio jogador seus ossos/equipamentos/efeitos, que ele já renderiza localmente, e mantém apenas um snapshot aguardando envio. A cópia para cada destinatário conserva os dados do outro jogador e não altera o estado autoritativo.

O controle atende comandos uma vez por quadro apresentado. `hm_native_sample` envia a pose remota e lê a captura local no mesmo comando, evitando duas esperas sequenciais por quadro. O loop desconta o tempo gasto em consultas de seu período de 16 ms. Quadros com ossos deixam de passar por uma segunda suavização de raiz de 35 ms; corpo e armas continuam usando o mesmo quadro.

## Corpo, cabelo e roupa

HMP2 distingue corpo e peça de cabeça/tronco/pernas. A prévia 14 usa as seis referências retidas pelo gerenciador de armadura do Link, valida geração/estado e obtém uma referência local temporária pelo resolvedor nativo. Não depende de `Actor::parent` nem percorre todos os atores do mundo para descobrir roupa. Uma referência negada é liberada e omitida; não há leitura por um ponteiro alternativo.

As paletas finais de cada peça são aplicadas por nome de osso e origem. Ossos compartilhados idênticos usam a paleta do corpo; os diferentes, incluindo ossos adicionais de cabelo/chapéu, mantêm sua própria transformação. Uma peça incompleta não descarta a pose corporal válida. Mantém o limite de 512 ossos. Não ativa uma segunda simulação Havok. Deformações feitas apenas nos vértices, sem alterar ossos, ainda não são transportadas.

O quadro `HMA1` carrega até 512 máscaras de visibilidade de osso/material, identificadas por nome e peça. O receptor usa os setters nativos e seus avisos de alteração, apenas quando o valor muda. Assim pode ocultar a pele/cabelo que o jogador local esconde ao equipar roupa. Esses estados permanecem no mesmo quadro discreto dos eventos/equipamentos; não são índices de memória copiados entre processos.

Roupas continuam vindo do inventário pessoal, tipos 4/5/6. Tinturas, expressões e todos os modelos extras/acessórios não estão integralmente reproduzidos.

## Efeitos de ataques — experimental

Além da API por nome, o hook comum registra o Event depois de inicializado e inserido no usuário visual, antes da escrita opcional do identificador de saída. Isso cobre pedidos diretos por descritor e pedidos sem saída. A API por nome e o hook comum são deduplicados por contexto/geração. Somente chaves e ciclo de vida viajam pela rede.

Até 256 condições numéricas dos quatro usuários ELink locais (corpo/espada/arco/escudo, até 64 cada) acompanham o quadro visual. O receptor resolve nome/tipo em suas próprias definições e aplica os setters inteiros/flutuantes, preservando a máscara nativa de alterações, antes de iniciar efeitos. Valores flutuantes não finitos e identidades duplicadas são recusados. Não ativa o controlador Player XLink nem eventos de combate/áudio.

Até 16 eventos são acompanhados, com encerramento e proteção de geração/sequência. `effects_applied` confirma um identificador nativo, **não uma imagem desenhada**. Rastros feitos por renderizadores próprios da arma, condições textuais, todos os estados temporários entre amostras e efeitos especiais fora desses usuários ainda não estão comprovados. O rastro de cada ataque precisa de teste no jogo.

## Itens e armas no mundo aberto

Com o servidor dedicado e os clientes da prévia 18, cada instância observa até 90 m ao redor do próprio Link. O registro conserva até 32.768 identidades, incluindo itens coletados. O cliente acompanha 32 próximos por vez e confirma a limpeza de cada slot antes de reutilizá-lo. Há até 16 descartes nativos próprios acompanhados por instância.

- Materiais de mapa das famílias de frutas, cogumelos, plantas, minérios, partes de monstros, carnes, assados, congelados, peixes e insetos com identidade de mapa válida.
- Armas comuns: famílias `Weapon_Sword_`, `Weapon_Lsword_`, `Weapon_Spear_`, `Weapon_Bow_`, `Weapon_Shield_`; exclui Master Sword e One-Hit Obliterator.
- **Descarte/arremesso normal de arma:** o hook registra o ator que o próprio jogo retirou do inventário, validando o proprietário e a geração da referência. Não retira a arma outra vez. O criador publica a posição; o outro cliente cria uma arma nativa com durabilidade e par de modificadores preservados.
- **Coleta dos dois lados:** a função nativa de concessão é adiada; o servidor reserva um único vencedor. O runtime verifica a entrada efetivamente adicionada ao inventário pessoal antes do recibo. Uma mensagem repetida não concede a arma novamente. Na concessão de armas, a durabilidade é o valor da entrada, não uma quantidade de itens.
- Capacidade esgotada: o descarte não ingressa no mundo compartilhado. Se sua coleta já havia sido adiada, o runtime recupera um único objeto físico local com os mesmos dados; não tenta um segundo crédito de inventário como recuperação. Uma falha nativa de recuperação conserva a pendência e é reportada; há no máximo três tentativas de criação, com limpeza do handle antes de outra tentativa.
- Quebra: apenas vida nativa zero de uma arma acompanhada produz cancelamento; desaparecimento/descarregamento não significa quebra. Uma coleta já reservada prevalece sobre quebra tardia.
- **Largar 1 maçã** continua disponível no launcher: reserva capacidade, cria objeto e confere a retirada de uma unidade antes de publicá-lo. Outros materiais descartados pelo menu normal continuam fora do fluxo novo.

Uma coleta anterior à primeira publicação permanece local. Depois da publicação, inclusive durante perda de heartbeat, a cópia acompanhada não pode contornar a reserva do servidor. Réplicas em criação assíncrona também ficam protegidas até inicialização.

Resultado ambíguo ou desconexão durante uma reserva não libera automaticamente o item para outro jogador. O estado incerto é persistido e conserva o bloqueio. Há até 32 pedidos pendentes. Pedidos finalizados são removidos após o reconhecimento do cliente, permitindo coletas posteriores na mesma conexão.

O servidor guarda os descartes aceitos no arquivo do mundo. Desconectar um cliente limpa suas réplicas locais. O próximo cliente pode reconstruir um objeto disponível a partir do registro. A permanência no inventário pessoal também depende do save do respectivo jogo.

Posições vêm do proprietário, com atualizações limitadas; rotação, trajetória física exata, colisões e transferência de autoridade ao carregar um objeto ainda não são compartilhadas. Também ficam fora comida preparada, pilhas, baús/lojas, itens de missão, frutos naturais sem identidade de mapa e espólios dinâmicos. A separação entre interiores está integrada, com gameplay geral ainda pendente.

## Dano e mundo

A identidade é hash do mapa + nome + posição original. Cada instância calcula a perda de vida local acumulada antes de aplicar o estado do servidor. O servidor soma as contribuições dos clientes e deduplica pelo contador de cada origem. Aplicar dano recebido não o transforma em outro golpe local.

O hook de remoção conserva evidência de vida realmente zerada antes que o ator desapareça. A captura admite monstros de mapa e árvores em pé com getter nativo de vida compatível; exclui raízes, galhos, tocos e troncos. A substituição por toco foi validada em dois jogos para `Obj_TreeConiferous_A_02` e `Obj_TreeBroadleaf_A_L`. Troncos e madeira continuam pendentes. Confirmações de vida zero podem ser reenviadas uma vez por segundo, respeitando o orçamento rotativo de quatro comandos, para liberar evidência capturada depois de uma escrita anterior. O registro conserva até 32.768 identidades na sessão persistente. Cada jogo observa até 64 próximas por amostra e recebe o conjunto necessário aos clientes carregados.

Horário e clima seguem o servidor dedicado, com um dia de 24 minutos por padrão. Dia, Blood Moon, missões, flags gerais, runas, projéteis, dano entre Links e todos os eventos do mundo ainda não estão integrados. O servidor coordena estado; não executa BOTW.

Vida e inventários dos jogadores continuam pessoais. Uma pose de ataque remota não aplica um golpe no outro mundo por si só. A integração nativa roda após os jobs dos atores; hooks de coleta/descarte/remoção apenas capturam ou enfileiram dados, sem chamar funções guest a partir dos workers.

## Atores do mundo

O servidor registra cada entidade por cenário, hash de mapa, nome e colocação original. Atribui a simulação ao cliente carregado mais próximo e aumenta a revisão quando muda essa atribuição. Os clientes recusam aplicar dados de um cenário diferente. A aplicação ocorre após os jobs dos atores, com referências nativas e geração conferidas. Cada captura comporta até 64 atores em 180 m.

Atores inicializados nos estados 1/2 entram com modelo presente e referência válida; a captura não exige que o culling local esteja ativo como no DemoNPC. Inicialização e remoção ficam excluídas.

São usados os monstros comuns, NPCs, objetos móveis da lista explícita e armas soltas do próprio jogo. Hash de mapa, nome e colocação original identificam o mesmo objeto entre processos. O despacho local de RootAI fica suspenso somente para um ator correspondente com estado recém-aplicado no cliente; o cliente responsável continua executando sua IA. Outros caminhos de física e atualização de NPCs inativos ainda podem avançar localmente. Expiração, desconexão e troca de host liberam referências e devolvem a autoridade local.

A paleta final dos modelos SDK reconhecidos é reposicionada junto da raiz recebida, preservando os deslocamentos relativos dos ossos. Isso corrige a separação entre o modelo e o esqueleto desenhado; não transmite toda a pose do NPC nem substitui sua física. Modelos/raízes desconhecidos e matrizes inválidas são omitidos.

**Atores precisam estar carregados nos dois jogos.** Ausentes são contados, não criados. Extras não são ocultados. Chefes, atores dinâmicos sem identidade de mapa, trajetórias físicas completas, combate contra o Link remoto, alvo mais próximo, agenda/diálogo exato de NPCs e persistência não estão implementados aqui. Animação é o pedido AS, não toda a fase/variáveis da árvore. Pegar/carregar um objeto no cliente pode sofrer correção pelo estado do host. As regras de coleta e descarte estão na seção de itens e armas.

## Diagnóstico e validação

O botão **Diagnóstico** do launcher coleta, em segundo plano, os estados nativos dos dois clientes e os inclui no próprio `diagnostico.txt`, sem congelar a janela. A coleta leve não lê memória privada de atores nem calcula hash das bibliotecas grandes. Há também telemetria `[hyrulem]` no log de cada processo enquanto a sessão está ativa. **Diagnosticar.cmd** gera `native-preview-diag.15`, incluindo:

- `armor_references`, `armor_head_bones`, `armor_upper_bones`, `armor_lower_bones`: peças encontradas nas referências reais e ossos próprios capturados.
- `visibility_captured`, `visibility_applied`, `effect_properties_captured`, `effect_properties_applied`: máscaras/condições capturadas e aplicações acumuladas.
- `skeleton_captured`, `skeleton_received`, `body_pose_applied`, `skeleton_applied`, `armor_source`: captura, chegada pela rede, aplicação ao corpo, aplicação total e origem das roupas.
- `pose_capture_status`, `pose_bones_skipped`: motivo da captura incompleta. A captura exige matrizes válidas e não publica um quadro corrompido.
- `drop_error`, `drop_inventory_before`, `drop_inventory_after`: motivo nativo e quantidades conferidas do descarte de maçã. Não confundir ausência de maçã com erro de rede.
- `world_role`, `world_epoch`, `world_captured`, `world_matched`, `world_missing`, `world_applied`, `world_ai_skipped`, `world_animations_applied`: papel (1 host/2 seguidor), época, captura, correspondência, ausentes, aplicações, decisões locais suspensas e pedidos AS aplicados. Ausente não é destruído.
- `pending_zero_events`: evidências de vida zero aguardando confirmação.
- `world_items_active`, `heartbeat_age_ms`, `source_materials`: modo ativo e observações.
- `native_drop_count`, `native_drops_active`, `native_drop_local_recovery`: descartes normais e recuperação local.
- `items_json`, `events_json`, `receipts_json`, `destroyed_json`: fontes, reservas, resultados e quebras.
- `enemies_json` com `damage`: vida e contador cumulativo observado.
- `replica_failures`, `pending_inventory_checks`: criação recusada e resultado de inventário ainda incerto.

Testes Go/TCP e race, testes C com memória sintética, emissão compilada dos hooks, 36 assinaturas únicas privadas e roundtrips do setter nativo foram executados. Builds/instalação Windows são verificados no CI. Isso não substitui jogar com dois clientes. `native_ready` permanece reservado à geração completa; não anuncia combate/IA/interação geral prontos.

[Auditoria](https://github.com/KnightCZS/HyruleM/blob/v0.5.0-preview.15/docs/AUDITORIA_0.5.0-preview.15.md) · [Pesquisa](RESEARCH_SYNC.md). Runtime Savage Wind, MIT. Sem código/BNP do Milk Bar, arquivos do jogo, saves, disassembly ou biblioteca recompilada no pacote.
