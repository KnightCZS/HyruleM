# HyruleM 0.5.0-preview.15 - candidata em validação

Primeira camada de mundo sob autoridade do anfitrião: posição, rotação e pedidos de animação dos atores de mapa já carregados nos dois jogos. Usa os atores nativos correspondentes, com IA local suspensa apenas no seguidor que recebeu estado recente. Até 64 atores em 180 m; veja os limites abaixo.

Os testes nativos e de transporte passaram. Candidatos anteriores foram testados com duas instâncias/GPU: corrigida a captura vazia, houve 26/26 correspondências, mas a conferência de posições reprovou com divergência de NPCs. O hook de IA foi movido para o despacho virtual comum; o teste seguinte ainda encontrou matrizes instáveis. A candidata atual conserva a trajetória da rede entre pacotes e confirma AS pela observação do pedido nativo, sem confiar em retorno booleano. Precisa repetir o teste GPU e não foi publicada. O último relato confirma evolução das marionetes, mas efeitos de espada/impacto e algumas roupas continuam pendentes.

## Atualizar e usar

1. Feche ambos os jogos e o launcher. Extraia o ZIP inteiro, incluindo `runtime-update`.
2. Execute **Iniciar-Nativo.cmd**, que instala o runtime com backup.
3. **Aguarde a recompilação da biblioteca privada do jogo com o gerador novo.** Há um hook novo; bibliotecas das prévias anteriores precisam ser regeneradas. A gamelib ABI continua 4, mas a ABI do adaptador mudou. Não altere `.id`.
4. Atualize os dois clientes e o servidor: **ABI 11 / protocolo 11**. O overlay de recursos da prévia 12 permanece compatível; é gerado localmente quando necessário. O ZIP não contém arquivos do jogo.
5. Hospede e conecte pelo IP/porta escolhidos. Na primeira abertura desta versão, **Itens do mundo: ligados** é ativado no painel de hospedagem. Depois disso a escolha é preservada; pode desligá-la antes de hospedar. O servidor separado usa `--shared-items=true` por padrão, ou `--shared-items=false` para desligar.
6. Para testar no mesmo PC, expanda **Testar dois jogos neste PC**. Cada instância tem save, cache, configurações e porta de controle próprios.

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

## Itens e armas — Hateno

O experimento cobre o mundo aberto, **até 90 m do ponto de encontro de Hateno**, e no máximo **32 identidades por sessão**, incluindo coletados/cancelados. Há até 16 descartes nativos próprios acompanhados por instância. Não é uma sincronização geral de todos os mapas.

- Materiais de mapa: `Item_Fruit_A`, `Item_Fruit_B`, `Item_Fruit_F`, `Item_Mushroom_A`, `Item_PlantGet_A`.
- Armas comuns: famílias `Weapon_Sword_`, `Weapon_Lsword_`, `Weapon_Spear_`, `Weapon_Bow_`, `Weapon_Shield_`; exclui Master Sword e One-Hit Obliterator.
- **Descarte/arremesso normal de arma:** o hook registra o ator que o próprio jogo retirou do inventário, validando o proprietário e a geração da referência. Não retira a arma outra vez. O criador publica a posição; o outro cliente cria uma arma nativa com durabilidade e par de modificadores preservados.
- **Coleta dos dois lados:** a função nativa de concessão é adiada; o servidor reserva um único vencedor. O runtime verifica a entrada efetivamente adicionada ao inventário pessoal antes do recibo. Uma mensagem repetida não concede a arma novamente. Na concessão de armas, a durabilidade é o valor da entrada, não uma quantidade de itens.
- Capacidade esgotada: o descarte não ingressa no mundo compartilhado. Se sua coleta já havia sido adiada, o runtime recupera um único objeto físico local com os mesmos dados; não tenta um segundo crédito de inventário como recuperação. Uma falha nativa de recuperação conserva a pendência e é reportada; há no máximo três tentativas de criação, com limpeza do handle antes de outra tentativa.
- Quebra: apenas vida nativa zero de uma arma acompanhada produz cancelamento; desaparecimento/descarregamento não significa quebra. Uma coleta já reservada prevalece sobre quebra tardia.
- **Largar 1 maçã** continua disponível no launcher: reserva capacidade, cria objeto e confere a retirada de uma unidade antes de publicá-lo. Outros materiais descartados pelo menu normal continuam fora do fluxo novo.

Uma coleta anterior à primeira publicação permanece local. Depois da publicação, inclusive durante perda de heartbeat, a cópia acompanhada não pode contornar a reserva do servidor. Réplicas em criação assíncrona também ficam protegidas até inicialização.

Resultado ambíguo ou desconexão durante uma reserva não libera automaticamente o item para outro jogador. Não há recuperação persistente dessas pendências. Há até 32 pedidos pendentes e 128 totais por conexão; exceder o total encerra a conexão com erro explícito.

**Recolha os descartes compartilhados antes de encerrar.** São objetos da sessão, sem persistência ou devolução automática. Encerrar limpa os descartes aceitos e as réplicas do mod; materiais originais e descartes que permaneceram locais não são apagados apenas por desligar o modo.

Posições vêm do proprietário, com atualizações limitadas; rotação, trajetória física exata, colisões e transferência de autoridade ao carregar um objeto ainda não são compartilhadas. Também ficam fora comida preparada, pilhas, baús/lojas, itens de missão, frutos naturais sem identidade de mapa, interiores e santuários.

## Dano e mundo

A identidade é hash do mapa + nome + posição original. Cada instância calcula a perda de vida local acumulada antes de aplicar o estado do servidor. O servidor soma as contribuições dos clientes e deduplica pelo contador de cada origem. Aplicar dano recebido não o transforma em outro golpe local.

O hook de remoção conserva evidência de vida realmente zerada antes que o ator desapareça. A captura admite monstros de mapa e árvores em pé com getter nativo de vida compatível; exclui raízes, galhos, tocos e troncos. **Zerar vida ainda não comprova a transição da IA de árvore para árvore caída/tronco**, e essa sequência não foi validada em gameplay. Não é correto anunciar o corte de árvores como resolvido somente por este mecanismo. Confirmações de vida zero podem ser reenviadas uma vez por segundo, respeitando o orçamento rotativo de quatro comandos, para liberar evidência capturada depois de uma escrita anterior. O registro de dano comporta 128 entidades; observações são limitadas a 64 próximas por amostra.

Horário/clima continuam sob autoridade do anfitrião, com sucessão e descarte de amostras antigas. Dia, Blood Moon, missões, flags gerais, runas, projéteis, dano entre Links e todos os eventos do mundo ainda não estão integrados. O servidor coordena estado; não executa BOTW.

Vida e inventários dos jogadores continuam pessoais. Uma pose de ataque remota não aplica um golpe no outro mundo por si só. A integração nativa roda após os jobs dos atores; hooks de coleta/descarte/remoção apenas capturam ou enfileiram dados, sem chamar funções guest a partir dos workers.

## Atores do mundo - novo na prévia 15

O primeiro jogador conectado é a autoridade; ao sair, o servidor elege outro e aumenta a época. Capturas antigas e publicações de seguidores são recusadas/descartadas. Até 64 atores elegíveis em 180 m do host são amostrados a 10 Hz e enviados somente ao outro jogador. A aplicação usa tempo decorrido, interpolação de posição/rotação e referências nativas com geração conferida.

Atores inicializados nos estados 1/2 entram com modelo presente e referência válida; a captura não exige que o culling local esteja ativo como no DemoNPC. Inicialização e remoção ficam excluídas.

São usados os monstros comuns, NPCs, objetos móveis da lista explícita e armas soltas do próprio jogo. Hash de mapa, nome e colocação original identificam o mesmo objeto entre processos. A IA original fica suspensa somente para um ator correspondente com estado recém-aplicado no cliente; o host continua executando sua IA. Expiração, desconexão e troca de host liberam referências e devolvem a autoridade local.

**Atores precisam estar carregados nos dois jogos.** Ausentes são contados, não criados. Extras não são ocultados. Chefes, atores dinâmicos sem identidade de mapa, trajetórias físicas completas, combate contra o Link remoto, alvo mais próximo, agenda/diálogo exato de NPCs e persistência não estão implementados aqui. Animação é o pedido AS, não toda a fase/variáveis da árvore. Pegar/carregar um objeto no cliente pode sofrer correção pelo estado do host. Coleta/descarte anteriores mantêm os limites de Hateno descritos acima.

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

[Auditoria](../docs/AUDITORIA_0.5.0-preview.15.md) · [Pesquisa](RESEARCH_SYNC.md). Runtime Savage Wind, MIT. Sem código/BNP do Milk Bar, arquivos do jogo, saves, disassembly ou biblioteca recompilada no pacote.
