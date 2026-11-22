# Prévia 20: validação em andamento

A compilação anterior ao vínculo assíncrono passou no CI (commit `fff241005583f1e47b05a4dadfb8946603886c54`). O teste do jogo confirmou a criação de um tronco após cortar uma árvore, mas a referência direta de parentesco veio vazia. Isso não foi contado como sincronização de espólios.

O novo adaptador acompanha o pedido nativo em três pontos verificados por assinatura única para 1.9.0: execução da RootAI, cópia dos argumentos da solicitação e chamada da fábrica. Uma tabela limitada carrega uma cópia da origem até a tarefa correta; o callback de inicialização exige o mesmo descritor e nome de ator. Não se associa origem por proximidade, nem se escreve nas estruturas de tarefas do jogo.

Verificações automatizadas concluídas:

- Solicitação e fábrica em threads distintas conservam a origem após invalidar a referência do ator original.
- Tarefa reutilizada, job aninhado sem ator válido, descritor ou nome diferentes e reinício não herdam o vínculo anterior.
- Os hooks de chamadas virtuais preservam a chamada original, endereço de retorno, registradores, SIMD e flags retornados pelo jogo.
- Fontes registradas fora dos 32 slots visuais podem liberar seus registros locais após descarregarem.
- Testes anteriores de reserva, crédito único, persistência, descarte de duplicatas e autorização de simulação permanecem na suíte.

Pendente: recompilar a biblioteca privada com os novos hooks e comprovar, com dois clientes e servidor separado, o ciclo completo árvore → tronco → madeira, migração de simulação e coleta única. Esta nota não certifica sincronização total do jogo.
