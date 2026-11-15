# HyruleM 0.5.0-preview.12

O jogador confirmou roupas e animações de espada/escudo/arco funcionando na prévia 11. Esta atualização reduz esperas na apresentação remota, captura paletas próprias de cabelo/roupa e adiciona um caminho experimental para eventos visuais ELink. **O resultado gráfico da prévia 12 ainda precisa de teste com dois jogos/GPU.**

## Atualizar e usar

1. Feche ambos os jogos e o launcher. Extraia o ZIP inteiro, incluindo `runtime-update`.
2. Execute **Iniciar-Nativo.cmd**, que instala o runtime com backup.
3. **Aguarde a recompilação da biblioteca privada:** os dois hooks novos de efeitos mudam a identidade do gerador. O gamelib continua ABI 4, mas a biblioteca das prévias 10/11 não contém os novos hooks. Não altere `.id` para contornar a verificação.
4. Atualize os dois clientes e o servidor: **ABI 9 / protocolo 9**. O overlay também é regenerado localmente para os usuários visuais de efeitos; não contém arquivos de jogo no ZIP.
5. Hospede e conecte pelo IP/porta escolhidos. Na primeira abertura desta versão, **Itens do mundo: ligados** é ativado no painel de hospedagem. Depois disso a escolha é preservada; pode desligá-la antes de hospedar. O servidor separado usa `--shared-items=true` por padrão, ou `--shared-items=false` para desligar.
6. Para testar no mesmo PC, expanda **Testar dois jogos neste PC**. Cada instância tem save, cache, configurações e porta de controle próprios.

## Atraso na visão do outro jogador

O servidor passa de snapshots padrão a cada 50 ms para 20 ms. O cliente troca a espera fixa de 100 ms por uma reserva adaptativa entre 20 e 100 ms: num fluxo sintético estável de 20 ms ela é de 25 ms. A reserva aumenta com jitter; não prevê ossos ou armas separadamente. Esses números descrevem o buffer, não uma medição de ping ou do atraso total no PC do jogador.

O servidor não devolve ao próprio jogador seus ossos/equipamentos/efeitos, que ele já renderiza localmente, e mantém apenas um snapshot aguardando envio. A cópia para cada destinatário conserva os dados do outro jogador e não altera o estado autoritativo.

O controle atende comandos uma vez por quadro apresentado. `hm_native_sample` envia a pose remota e lê a captura local no mesmo comando, evitando duas esperas sequenciais por quadro. O loop desconta o tempo gasto em consultas de seu período de 16 ms. Quadros com ossos deixam de passar por uma segunda suavização de raiz de 35 ms; corpo e armas continuam usando o mesmo quadro.

## Corpo, cabelo e roupa

HMP2 acrescenta à identidade de cada osso a origem: corpo ou peça equipada de cabeça/tronco/pernas. A captura lê também as paletas finais dos atores de armadura ativos e ligados ao Link local. Ossos compartilhados idênticos usam a paleta do corpo; transformações próprias da peça e ossos extras são transmitidos separadamente. Isso evita que um osso do corpo substitua um osso de cabelo/chapéu de mesmo nome.

Não ativa um segundo Havok nem colisões nas marionetes: reproduz as transformações finais calculadas no cliente de origem. O relatório expõe `armor_head_bones`, `armor_upper_bones` e `armor_lower_bones`. Zero pode indicar uma peça rígida, paleta indisponível ou ator que não corresponde ao filtro; precisa ser interpretado junto do equipamento usado. O limite permanece em 512 ossos. Uma paleta de peça inválida/excessiva é omitida inteira, preservando o corpo válido e registrando `piece-pose-unavailable`; uma paleta corporal inválida continua recusada.

Roupas continuam vindo do inventário pessoal (tipos 4/5/6); categorias de arma convertem espada/escudo/arco para espada/arco/escudo na rede. Tinturas, expressões, bainhas e variações especiais não são integralmente reproduzidas. As marionetes continuam DemoNPCs visuais.

## Efeitos de ataques — experimental

Um hook somente de leitura registra pedidos de efeito por nome depois que o ELink do corpo/armas locais retorna um identificador válido. Outro registra seu encerramento. Só chaves, origem, idade e sequência viajam pela rede; ponteiros e recursos permanecem em cada processo. A reprodução usa o usuário visual ELink da marionete/arma correspondente, depois de aplicar sua pose. Áudio e o controlador de propriedades Player XLink continuam desativados; não executa eventos de dano/AI.

Há até 16 eventos acompanhados, encerramento por ausência/limpeza e proteção por geração dos identificadores e sequência. Pedidos repetidos não reiniciam as partículas. `effects_source_users`, `effects_captured`, `effects_applied`, `effects_stopped`, `effects_missing_user`, `effects_overflow` e `effect_last_key` mostram onde o fluxo parou. `effects_applied` confirma um identificador nativo, não uma imagem renderizada.

**O rastro do spin attack ainda precisa de confirmação no jogo.** O caminho cobre pedidos ELink por nome; efeitos acionados diretamente por outro caminho, fitas/shaders próprios da arma, condições elementais e todos os efeitos especiais não estão comprovados. Não basta o teste de socket para declarar esses visuais resolvidos.

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

Horário/clima continuam sob autoridade do anfitrião, com sucessão e descarte de amostras antigas. Dia, Blood Moon, missões, flags gerais, posição/IA dos monstros, runas, projéteis, dano entre Links e todos os eventos do mundo ainda não estão integrados. O servidor coordena estado; não executa BOTW.

Vida e inventários dos jogadores continuam pessoais. Uma pose de ataque remota não aplica um golpe no outro mundo por si só. A integração nativa roda após os jobs dos atores; hooks de coleta/descarte/remoção apenas capturam ou enfileiram dados, sem chamar funções guest a partir dos workers.

## Diagnóstico e validação

O botão **Diagnóstico** do launcher coleta, em segundo plano, os estados nativos dos dois clientes e os inclui no próprio `diagnostico.txt`, sem congelar a janela. A coleta leve não lê memória privada de atores nem calcula hash das bibliotecas grandes. Há também telemetria `[hyrulem]` no log de cada processo enquanto a sessão está ativa. **Diagnosticar.cmd** gera `native-preview-diag.12`, incluindo:

- `skeleton_captured`, `skeleton_received`, `body_pose_applied`, `skeleton_applied`, `armor_source`: captura, chegada pela rede, aplicação ao corpo, aplicação total e origem das roupas.
- `pose_capture_status`, `pose_bones_skipped`: motivo da captura incompleta. A captura exige matrizes válidas e não publica um quadro corrompido.
- `drop_error`, `drop_inventory_before`, `drop_inventory_after`: motivo nativo e quantidades conferidas do descarte de maçã. Não confundir ausência de maçã com erro de rede.
- `pending_zero_events`: evidências de vida zero aguardando confirmação.
- `world_items_active`, `heartbeat_age_ms`, `source_materials`: modo ativo e observações.
- `native_drop_count`, `native_drops_active`, `native_drop_local_recovery`: descartes normais e recuperação local.
- `items_json`, `events_json`, `receipts_json`, `destroyed_json`: fontes, reservas, resultados e quebras.
- `enemies_json` com `damage`: vida e contador cumulativo observado.
- `replica_failures`, `pending_inventory_checks`: criação recusada e resultado de inventário ainda incerto.

Testes Go/TCP e race, testes C com memória sintética, emissão compilada dos hooks, 29 assinaturas únicas privadas e roundtrips do setter nativo foram executados. Builds/instalação Windows são verificados no CI. Isso não substitui jogar com dois clientes. `native_ready` permanece reservado à geração completa; não anuncia combate/IA/interação geral prontos.

[Auditoria](../docs/AUDITORIA_0.5.0-preview.12.md) · [Pesquisa](RESEARCH_SYNC.md). Runtime Savage Wind, MIT. Sem código/BNP do Milk Bar, arquivos do jogo, saves, disassembly ou biblioteca recompilada no pacote.
