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


## Retomada de 22/11: teste real e correções

O runtime do commit `1120632c` foi instalado e sua biblioteca privada reconstruída no PC de teste. Ao cortar `Obj_TreeBroadleaf_A_L`, o jogo criou o tronco, mas o diagnóstico não identificou o criador. O caso não passou como sincronização de espólios.

A leitura do copiador nativo mostrou que ele guarda o nome da SafeString em pedido +8 na tarefa +0xc0. O adaptador lia pedido +0x18, outra SafeString de parâmetros. A fixture anterior repetia o mesmo layout incorreto. A fixture corrigida, com nome e parâmetro distintos, falha antes da correção e passa depois. A confirmação em dois jogos permanece pendente.

O dedicado também conserva um simulador válido enquanto outro cliente não estiver mais de dois metros mais próximo da entidade. Isso estabiliza a autoridade, sem definir o alvo de combate da IA. Pausa, observação expirada, descarregamento, mudança de cenário e desconexão continuam liberando a autoridade imediatamente. A regressão reproduziu a oscilação antes da mudança, e a suíte Go interna passou com detector de concorrência.

O CI também expôs uma leitura intermitente de código de saída 0 no teste de carregamento do runtime. A reprodução no Windows PowerShell 5.1 retornou 0 em uma de vinte execuções com `Start-Process -PassThru`, enquanto as vinte execuções com `System.Diagnostics.Process` iniciado pelo próprio objeto retornaram o código esperado 73. O instalador passou a conservar esse handle, mantendo o timeout e a restauração em caso de falha.
