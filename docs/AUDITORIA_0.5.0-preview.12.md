# Auditoria da prévia 12

## Evidência recebida

O jogador confirmou roupas e animações de espada/escudo/arco sincronizadas na prévia 11. O relatório mostra ABI 8 nos dois runtimes, estado ativo, 121 ossos capturados/recebidos/aplicados no corpo e 209/205 aplicações no total com equipamento. Não há evidência de fallback ou perda de Skeleton nessa execução. Relata pequeno atraso na apresentação, cabelo/chapéu rígidos em algumas ações e falta do efeito do spin attack. O arquivo privado não foi publicado.

## Alterações

O buffer remoto fixo de 100 ms é substituído por uma reserva calculada dos intervalos recentes e jitter, limitada a 20–100 ms. Um fluxo sintético de snapshots a cada 20 ms dá 25 ms de reserva. O servidor usa 20 ms por padrão. Não há alegação de ping real, FPS ou redução medida do atraso total no computador do jogador.

O servidor não devolve ao próprio jogador seus ossos/equipamentos/efeitos, que ele já renderiza localmente, e mantém apenas um snapshot aguardando envio. A cópia para cada destinatário conserva os dados do outro jogador e não altera o estado autoritativo.

A porta de controle drena comandos por quadro apresentado. Antes, ler a captura e enviar a pose exigiam duas consultas sequenciais. `hm_native_sample` valida/enfileira o remoto e copia a captura local em uma única resposta; não faz chamadas guest na thread de controle. O loop desconta consultas de seu período de 16 ms. O runtime não aplica a segunda suavização exponencial de raiz a quadros esqueléticos já interpolados.

HMP2 identifica ossos por peça e CRC do nome. A captura do corpo é complementada por paletas de armadura ativa ligada ao Link local, sob o lock nativo da lista de atores. Ossos compartilhados idênticos são herdados do corpo; os diferentes e extras conservam a origem. Não instancia outro Havok nem habilita colisões nas marionetes. O limite continua em 512 ossos e matrizes inválidas continuam recusadas.

Eventos visuais ELink: observação depois do retorno do pedido por nome, filtrada pelos usuários visuais do jogador/armas locais; captura do encerramento; transporte de chave, origem, sequência e idade; reprodução nos usuários visuais das marionetes e limpeza de identificadores. A geração de EventHandle está em Event+0x20, diferente do AssetExecutorHandle (0x10). O encerramento verifica a geração antes de chamar o método nativo. Replays não reiniciam eventos; histórico por sequência evita reexecução depois de reutilizar um slot. Há até 16 eventos acompanhados. Áudio, controlador de propriedades Player XLink, dano e IA não são encaminhados.

O overlay retém apenas usuários ELink visuais apropriados ao corpo/equipamento, mantém DemoNPC e os controladores de combate/física desativados e é gerado nos arquivos do jogador. Diagnóstico mostra ossos extras por peça e etapas/capacidade dos efeitos.

## Verificação

- Go: fluxo contínuo de captura → bridge → TCP → cliente → comando combinado, preservando ossos por peça e eventos; coerência temporal com corpo/arma; limites/rejeição de eventos; reserva LAN/jitter; testes anteriores de itens, inventário e mundo. Race detector e vet.
- C sintético: corpo e armadura com ossos de mesmo nome e transformações diferentes, isolamento de pai, herança e aplicação por peça; eventos locais/remotos, preservação de CPU, emissão única, encerramento, geração reciclada e histórico de sequência.
- Emissão de sete pontos de hook compilada/executada com fixtures sintéticas. Funções privadas que contêm os dois novos hooks e o callback de mundo regeneradas/compiladas com e sem registradores locais.
- Na execução privada de trechos originais 1.9.0 em Unicorn, dependências internas de lookup/emissão são sintéticas: confirma os registradores de retorno e o contrato do identificador. O encerramento real executado escreve a flag esperada. Isso não confirma lookup de recursos nem desenho de partículas.
- Assinaturas versionadas únicas no módulo privado e aplicação idempotente do adaptador. CI Windows verifica launcher, build do runtime/gerador, pacote e instalador. ZIP é conferido por CRC, hashes e exports antes da entrega.

Sem gameplay/GPU da prévia 12, medição real de latência ou confirmação de todos os chapéus/armaduras e do rastro do spin attack. O caminho novo cobre pedidos ELink por nome; fitas/shaders de arma e outros caminhos de efeitos não estão comprovados. Contadores nativos não substituem imagens do jogo.

## Atualização e escopo

ABI 9, protocolo 9, overlay novo. Gamelib permanece ABI 4, mas os dois hooks novos alteram a identidade do gerador: aguardar regeneração/recompilação nos clientes; não editar `.id`. Nada altera main ou a release estável nesta publicação experimental.

Mundo compartilhado mantém Hateno/90 m/32 identidades, sem persistência. Não inclui combate entre Links, autoridade/posição/IA de monstros, runas/projéteis, baús/flags gerais ou árvores caídas confirmadas. O servidor coordena snapshots/transações e não executa BOTW. Nenhum código de decompilação, BNP do Milk Bar, arquivo de jogo, save, disassembly ou código gerado é publicado.
