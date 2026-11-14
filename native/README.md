# HyruleM 0.5.0-preview.4

A prévia 3 teve o modelo remoto visível confirmado pelo usuário, mas ainda parado/em T-pose. Esta atualização adiciona captura dos pedidos AS reais do jogador local, transmissão dos canais de tronco/pernas e reinício de ações, direção real e atualização das duas cópias da transformação do ator.

O proxy deriva folhas esqueléticas genéricas dos recursos privados da instalação e remove seletores e eventos de dano dependentes do Player. A reprodução usa clipes representativos; fidelidade de variações de arma/velocidade/equipamento ainda é parcial. O cabelo e os equipamentos não foram implementados. Mostrar o golpe não aplica dano entre jogadores.

## Instalar

1. Feche os dois jogos e o launcher antigo. Extraia o ZIP novo inteiro.
2. Execute **Iniciar-Nativo.cmd**, que instala/confere o runtime configurado e o gerador com **ABI 3**, com backup e restauração em falha.
3. **Reconstrua a biblioteca do jogo** na configuração do Savage Wind. O novo hook de captura só existe na biblioteca regenerada. Não edite o arquivo `.id`.
4. Use o launcher deste pacote e o servidor novo: **protocolo 4**, nos dois clientes. O overlay privado será atualizado antes de abrir os jogos.
5. Conecte ambos no mundo aberto. Entrar preserva a posição; **Reunir em Hateno** é manual.

O EXE dentro do pacote ativa o modo nativo automaticamente; sua instalação é conferida antes de preparar o jogo. Um pacote misturado ou runtime antigo é rejeitado explicitamente.

## Testar e diagnosticar

Mova um jogador, vire sem andar, pule e execute ataques consecutivos; observe no outro cliente e repita no sentido inverso. Esta atualização ainda precisa dessa confirmação visual: os testes de código e recursos não substituem gameplay.

**Diagnosticar.cmd** consulta os dois clientes, apenas em leitura. Os relatórios `native-preview-diag.4` incluem `hm_local_state` (canais AS, sequências e contador de captura) e `hm_native_status` (posição desejada/observada e pedidos de animação). `native_ready` continua reservado à Gen1 completa; as funções de combate/IA ainda não estão integradas.

## O que compartilha

- Vida dos monstros do mapa observados a até 140 m, no máximo 64 por leitura e 128 por sessão.
- Menor vida conhecida pelo servidor, incluindo zero. Não sincroniza ainda animação de morte, movimento ou autoridade da IA.
- Só um monstro carregado com o mesmo hash, nome e posição original recebe o valor. Endereços e IDs de processo nunca viajam pela rede.
- Saves, vida e inventário de cada jogador permanecem separados. O pacote não contém jogo, modelos, saves ou biblioteca recompilada.

Teste no mundo aberto. Missões, baús, itens no chão, árvores, clima, horário, Blood Moon e mudanças de mapa não são sincronizados. Reiniciar o servidor inicia outra sessão de estado compartilhado.

## Validação

- Callback real e armazenamento de HashId conferidos no módulo fornecido; quatorze assinaturas únicas.
- Testes C para controlador/dono, CPU, referências, criação/remoção e aplicação de vida somente na thread do jogo ao monstro correspondente.
- Testes com dois clientes para dano/vida zero convergindo, observações atrasadas sem cura e identidade inválida.
- Recursos derivados privadamente da instalação, preservando registros e subarquivos originais, sem controlador de física/dano do Player no proxy.

Pesquisa: [RESEARCH_SYNC.md](RESEARCH_SYNC.md). Runtime base [Savage Wind](https://github.com/Revivedx/SavageWindRecomp/tree/740b18025fe11ba03c117dbb56668db304b5043e), licença MIT. Nenhum código ou asset do Milk Bar foi incorporado.
