# Auditoria 0.5.0-preview.10 — issue #7

## Evidência e causa

Os dois relatórios anexados ao issue #7 usam ABI 6, ator ativo sem fallback, quatro peças visuais e somente Armor_Default para roupa. Isso comprova o caminho de atores, não que o jogador estivesse realmente sem roupas. O getter anterior consultava atores de equipamento para os seis slots; agora o inventário equipado fornece as roupas. Os relatórios também não tinham uma sessão de materiais aberta. Na prévia 9, o recurso vinha desligado e não abrangia armas ou árvores.

O corpo anterior vinha de uma redução heurística do grafo AS Player a um clipe esquelético. A arma já copiava a matriz real: as fontes podiam representar ataques diferentes. A prévia 10 transmite matrizes finais dos ossos, relativas à raiz, e mantém a mesma interpolação do corpo/arma. O grafo AS original não é executado em nome de outro jogador; assim, uma animação visual não reproduz seus eventos de jogo automaticamente.

## Implementação e limites

- HMP1: até 512 ossos, CRC32 do nome e matriz 3x4 float32, limites/tamanho/campos reservados/duplicatas/NaN verificados. Quadro completo validado antes da publicação. Canal local até 65535 bytes; protocolo de rede até 256 KiB.
- SDK nativo: métodos virtuais conferidos com sites da versão antes de ler a paleta ou chamar seu setter. Captura remove a raiz e aplicação recompõe a raiz remota. Armadura lê pouch equipado sob lock nativo, com leitura limitada e cache de 200 ms.
- Dano: contadores por origem/entidade, soma de deltas e exclusão de escritas recebidas. Hook de remoção exige vida efetivamente zero; ausência nunca vira destruição. A transição de IA/queda de árvores permanece sem confirmação.
- Armas: identidade fraca com geração e proprietário local no hook de drop/throw. O jogo já debitou o inventário; não há débito adicional. Durabilidade e par modificador/valor via caminho nativo. A concessão passa uma cópia temporária do par porque a função nativa o consome/zera.
- Coleta: reserva exclusiva e comparação antes/depois de entradas pessoais, sem somar durabilidade como quantidade. Uma concessão ambígua não é retentada. Recebimento repetido não duplica item.
- Corridas: pickup antes da publicação é local e cancela a fonte; depois da publicação não contorna autoridade mesmo sem heartbeat. Handles de réplicas e de recuperação são protegidos durante a criação. Recusa por capacidade mantém o original local ou recupera exatamente um ator físico; não faz reembolso incerto no inventário.
- Quebra: proprietário só cancela uma arma disponível mediante vida zero capturada. Uma reserva já aceita prevalece sobre uma quebra tardia. Descarregamento não é quebra.
- Escopo: Hateno/90 m, 32 identidades incluindo tombstones, 16 descartes próprios, 32 pedidos pendentes/128 totais por conexão. Sem persistência/recuperação de reserva incerta após reconexão, sem física/rotação geral compartilhada e sem combate Player remoto.

O pedido de vários Links completos permanece maior que esta atualização: faltam autoridade/transferência de entidades, execução idempotente dos eventos das ações, projéteis/runas, física, IA e persistência. Uma DLL adicional não entrega esses mecanismos. O módulo nativo já é integrado ao runtime que carrega a biblioteca privada do jogo; as chamadas estão na barreira de jobs e preservam a CPU guest.

## Verificação realizada

- Go: codec/limites de 512 ossos, mensagens fragmentadas, relógio comum de arma/corpo e matrizes degeneradas; dois clientes TCP somando dano, repetição e regressão de contador; arma de convidado com condição/modificadores, exclusividade, quebra tardia e capacidade; testes anteriores, detector de corrida e vet.
- C sintético: captura/aplicação por nomes, transformação da raiz, troca inválida atômica, fonte de armadura pessoal e CPU preservada. Drop nativo com geração/proprietário, pickup antes/depois da publicação, concessão exata uma vez, inventário cheio, recuperação por capacidade e proteção do handle. Recuperação não duplica após pickup local. Vida zero e exclusão do dano aplicado pelo servidor.
- Hooks emitidos e compilados: barreira, pickup, remoção, drop e throw; instrução substituída e registradores preservados. Regeneração privada dos trechos novos com/sem registradores locais.
- 26 assinaturas únicas no main privado 1.9.0. Getter/setter esquelético nativo executado em Unicorn com 12 rotações/translações: layout, flags de atualização, contagem e osso vizinho preservado. Nenhum conteúdo Nintendo incluído no repositório.
- Windows: compilação, empacotamento e backup/rollback do instalador pelo workflow nativo. Confira a execução associada ao commit da release.

**Não realizado:** gameplay/GPU desta versão, medição de FPS, validação da fase final de renderização, corte/queda de árvores e concessão de armas em uma partida real. Não anunciar estes resultados como confirmados. Issue #7 permanece aberto até validação.

Pesquisa: nota original do projeto, documentação/código do Savage Wind fixado em 740b18025fe11ba03c117dbb56668db304b5043e e estudo conceitual do Milk Bar. Nenhum código nem BNP do Milk Bar foi incorporado.
