# HyruleM 0.5.0-preview.9

Launcher reorganizado, correções da revisão e primeiro experimento de materiais compartilhados. A visibilidade/movimento dos dois Links foi confirmada pelo jogador na prévia 7. Equipamentos da 8 e a interação com materiais da 9 **ainda não foram confirmados em gameplay**. Testes sintéticos, testes Windows e análise das assinaturas não substituem esse teste.

## Atualizar

1. Feche ambos os jogos e o launcher. Extraia o ZIP inteiro, inclusive `runtime-update`.
2. Execute **Iniciar-Nativo.cmd**. O instalador atualiza o runtime configurado com backup.
3. **Aguarde a regeneração/compilação da biblioteca do jogo.** A prévia 9 adiciona um hook de coleta e três sites; as bibliotecas geradas pelas prévias 7/8 não servem para esta integração. Não edite `.id` para contornar a verificação.
4. Atualize os dois clientes e o servidor: **ABI 6 / protocolo 6**. O overlay visual da prévia 8 pode ser reutilizado; os recursos continuam derivados localmente da instalação do proprietário.

## Launcher

- **Início:** multiplayer em destaque, campanha offline e sandbox.
- **Sandbox:** região inicial, save base, perfil de missões e ferramentas em jogo.
- **Configurações:** executável, dados, preparação e instalação.
- **Multiplayer:** IP/porta para conectar e painel separado para hospedar. A porta do servidor local tem campo próprio. Nome, conexão e estado ficam no cartão do jogador.
- **Testar dois jogos neste PC:** expande o segundo cliente e o botão de organizar janelas. As instâncias mantêm saves, caches e portas locais próprios.
- Fechar o menu mantém a conexão; sair do launcher encerra a rede.

## Materiais de Hateno — experimento inicial

A opção **Materiais: experimental** fica desligada inicialmente. Ative no painel de hospedagem **antes** de iniciar o servidor; o cliente convidado recebe a configuração. No servidor independente use `--shared-items`. O escopo padrão é o mundo aberto de Hateno, até 90 m do ponto de encontro.

Nesta primeira etapa entram apenas `Item_Fruit_A`, `Item_Fruit_B`, `Item_Fruit_F`, `Item_Mushroom_A` e `Item_PlantGet_A`: três frutas, um cogumelo e uma erva identificados pelos IDs do jogo. A implementação:

1. O anfitrião publica materiais observados do mapa com hash/nome/posição original. Ausência em uma leitura não significa coleta.
2. Materiais largados pelo inventário de qualquer jogador são publicados pelo dono. **Afaste-se pelo menos 1,5 m** para o objeto sair da região das mãos; a leitura ocorre fora do menu/pausa.
3. Os outros clientes associam seu objeto correspondente ou criam uma réplica nativa usando os recursos originais da própria instalação. Réplicas próprias são reconhecidas inclusive durante a criação assíncrona e não são anunciadas como novos drops.
4. O hook intercepta a concessão nativa do material e enfileira um pedido. O servidor reserva o objeto para um único jogador, validando alcance/estado recente.
5. Após os jobs, o runtime confere a quantidade no inventário, solicita **uma unidade** pela função nativa e confere a diferença. O recibo confirma a coleta; o objeto some nos outros clientes. Um recibo repetido não concede outra unidade.
6. Falha confirmada libera o objeto. Resultado ambíguo ou desconexão durante uma reserva **não libera o item automaticamente para outra pessoa**: a sessão conserva a pendência para impedir concessão duplicada.

O registro é limitado a **32 identidades por sessão**, incluindo objetos já coletados. A reserva e os tombstones sobrevivem à troca de anfitrião e à entrada tardia, enquanto o servidor continuar aberto. O registro não é persistido em disco nem recuperado por perfil após reconexão; para uma reserva incerta a recuperação automática ainda não está implementada.

**Limites:** não inclui armas/durabilidade, modificadores, comida preparada, itens de missão, pilhas múltiplas por objeto, coleta em lojas/baús, toda a física de objetos, arremesso de um objeto do mapa com transferência de autoridade, cutscenes/fanfarras de primeira coleta ou interiores/santuários. Posições dos objetos do mapa seguem o anfitrião; drops seguem quem os originou. Objetos queimados/consumidos por física não são um evento de coleta confirmado. O raio de Hateno é uma restrição inicial, não uma identidade geral de todos os mapas. **Esse fluxo exige validação real antes de ampliar o escopo.**

Saves, vida e inventário seguem pessoais. Só o vencedor de uma coleta confirmada recebe aquela unidade. Encerrar o experimento limpa réplicas pertencentes ao mod e solta referências dos objetos originais; não apaga os objetos originais apenas para desligar o modo.

## Presença, aparência e ambiente

A criação/movimento/remoção dos Links continua no callback posterior à espera dos jobs, com referências e verificações nativas. Os corpos e as seis peças visuais são DemoNPCs sem controlador Player de física/dano. As roupas/armas são aparência e não concessões de inventário.

Mantêm-se os slots AS, a suavização de posição/rotação e o Morph do clipe representativo. A deduplicação agora usa nomes exatos de locomoção, e o prazo do equipamento é renovado enquanto ativo/pausado. Fase/velocidade exatas de clipes, tinturas, máscaras especiais, capas/bainhas e expressões ainda não estão completas.

O primeiro jogador é o anfitrião de horário/clima; há sucessão e descarte de amostras antigas. A restauração das políticas agora pode retentar uma falha parcial. O clima é global do anfitrião, sem previsão por região. Dia, Blood Moon, missões, IA e eventos completos continuam locais. O servidor não executa BOTW.

A vida de monstros de mapa ainda converge para a menor observação válida, por identidade de mapa e posição original; isso não fornece movimento, alvo ou IA compartilhados.

## Diagnóstico

**Diagnosticar.cmd** gera dois relatórios `native-preview-diag.9`. Inclui capacidades, fase do callback, referências, estado dos atores/peças, horário/clima e `hm_items_status`:

- `source_materials`: materiais locais observados.
- `replica_handles` / `replica_failures`: criações e falhas de réplicas.
- `pickup_intercepts`: entradas que passaram pelo hook de coleta.
- `events_json` / `receipts_json`: pedidos e resultados verificáveis.
- `pending_inventory_checks`: resultado de quantidade ainda ambíguo.

Um handle/modelo ativo não comprova desenho na GPU. `native_ready` permanece reservado à Gen1 completa; combate, coleta geral e IA ainda não estão finalizados.

## Validação

Testes C de CPU/barreira/referências, coleta adiada, quantidade exata, repetição de recibo, limpeza e resultados ambíguos; dois fragmentos de hooks compilados; **19 assinaturas únicas** verificadas no módulo privado 1.9.0; testes Go com dois clientes TCP, disputa de coleta, falha de inventário, reconexão/entrada tardia, limites e orçamento de comandos. Windows valida UI, backup/rollback e compilação. As capturas da interface usam dados sintéticos, sem jogo aberto.

[Auditoria desta atualização](../docs/AUDITORIA_0.5.0-preview.9.md) · [Pesquisa](RESEARCH_SYNC.md). Runtime base: Savage Wind, MIT. Sem código/BNP do Milk Bar, sem arquivos do jogo, saves ou biblioteca recompilada no download.
