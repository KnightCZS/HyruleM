# HyruleM 0.5.0-preview.10

Correções motivadas pelo [relatório #7](https://github.com/KnightCZS/HyruleM/issues/7): pose real em vez de clipe aproximado, roupas equipadas pelo inventário pessoal, dano cumulativo e primeira integração de descarte/arremesso normal de armas. O jogador confirmou presença/movimento e fluidez na versão anterior; **esta nova integração ainda precisa de teste com dois jogos e GPU**. Não é uma implementação completa de todas as ações de Link.

## Atualizar e usar

1. Feche ambos os jogos e o launcher. Extraia o ZIP inteiro, incluindo `runtime-update`.
2. Execute **Iniciar-Nativo.cmd**, que instala o runtime com backup.
3. **Aguarde regenerar e recompilar a biblioteca do jogo.** Há novos hooks e assinaturas. Não altere `.id` para pular a verificação.
4. Atualize os dois clientes e o servidor: **ABI 7 / protocolo 7**. O overlay de aparência da prévia 8 permanece compatível; seus recursos continuam gerados localmente.
5. Hospede e conecte pelo IP/porta escolhidos. Na primeira abertura desta versão, **Itens do mundo: ligados** é ativado no painel de hospedagem. Depois disso a escolha é preservada; pode desligá-la antes de hospedar. O servidor separado usa `--shared-items=true` por padrão, ou `--shared-items=false` para desligar.
6. Para testar no mesmo PC, expanda **Testar dois jogos neste PC**. Cada instância tem save, cache, configurações e porta de controle próprios.

## Corpo, armas e roupas

A versão anterior escolhia um clipe esquelético representativo do grafo Player, enquanto a arma usava a transformação real do jogador. Isso explica movimentos diferentes entre corpo e espada. O novo caminho captura a paleta final dos ossos do modelo local, identifica os ossos por CRC do nome e aplica a pose pelo setter nativo do modelo remoto. Corpo, roupa e transformações das armas usam o mesmo intervalo de interpolação. Matrizes singulares fazem o quadro visual completo avançar junto, sem separar a arma do corpo.

O formato HMP1 contém até 512 ossos com matrizes relativas à raiz; não transporta endereços, recursos nem arquivos do jogo. O setter e seus layouts foram conferidos no módulo privado 1.9.0, inclusive executando as instruções nativas em memória sintética. Isso comprova os getters/setters e os limites, **não comprova o resultado final do renderer**. Verifique `skeleton_captured` e `skeleton_applied` nos relatórios. Se não houver ossos compatíveis, o caminho AS antigo continua como contingência aproximada.

As roupas agora vêm das entradas válidas/equipadas do inventário pessoal. O getter de atores de equipamento continua sendo usado para as armas. Tinturas, expressões, efeitos, bainhas e variações especiais ainda não são reproduzidos integralmente. As marionetes continuam DemoNPCs visuais, sem controlador Player completo.

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

O hook de remoção conserva evidência de vida realmente zerada antes que o ator desapareça. A captura admite monstros de mapa e árvores em pé com getter nativo de vida compatível; exclui raízes, galhos, tocos e troncos. **Zerar vida ainda não comprova a transição da IA de árvore para árvore caída/tronco**, e essa sequência não foi validada em gameplay. Não é correto anunciar o corte de árvores como resolvido somente por este mecanismo. O registro de dano comporta 128 entidades; observações são limitadas a 64 próximas por amostra.

Horário/clima continuam sob autoridade do anfitrião, com sucessão e descarte de amostras antigas. Dia, Blood Moon, missões, flags gerais, posição/IA dos monstros, runas, projéteis, dano entre Links e todos os eventos do mundo ainda não estão integrados. O servidor coordena estado; não executa BOTW.

Vida e inventários dos jogadores continuam pessoais. Uma pose de ataque remota não aplica um golpe no outro mundo por si só. A integração nativa roda após os jobs dos atores; hooks de coleta/descarte/remoção apenas capturam ou enfileiram dados, sem chamar funções guest a partir dos workers.

## Diagnóstico e validação

**Diagnosticar.cmd** gera `native-preview-diag.10`, incluindo:

- `skeleton_captured`, `skeleton_applied`, `armor_source`: captura/aplicação de ossos e origem das roupas.
- `world_items_active`, `heartbeat_age_ms`, `source_materials`: modo ativo e observações.
- `native_drop_count`, `native_drops_active`, `native_drop_local_recovery`: descartes normais e recuperação local.
- `items_json`, `events_json`, `receipts_json`, `destroyed_json`: fontes, reservas, resultados e quebras.
- `enemies_json` com `damage`: vida e contador cumulativo observado.
- `replica_failures`, `pending_inventory_checks`: criação recusada e resultado de inventário ainda incerto.

Testes Go/TCP e race, testes C com memória sintética, emissão compilada dos hooks, 26 assinaturas únicas privadas e roundtrips do setter nativo foram executados. Builds/instalação Windows são verificados no CI. Isso não substitui jogar com dois clientes. `native_ready` permanece reservado à geração completa; não anuncia combate/IA/interação geral prontos.

[Auditoria](../docs/AUDITORIA_0.5.0-preview.10.md) · [Pesquisa](RESEARCH_SYNC.md). Runtime Savage Wind, MIT. Sem código/BNP do Milk Bar, arquivos do jogo, saves, disassembly ou biblioteca recompilada no pacote.
