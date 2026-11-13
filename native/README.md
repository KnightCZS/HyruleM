# HyruleM 0.5.0-preview.2

Corrige a rotina que ignorava os pedidos de criação do Link remoto: o callback recebe um controlador de dano, cujo dono fica em `this + 0x18`. A versão anterior comparava o controlador diretamente com o jogador e saía antes da fila. A relação foi conferida no módulo 1.9.0 e os testes reproduzem agora a chamada real.

Inclui a primeira integração de mundo: convergência de vida dos monstros próximos, identificados por hash do mapa, nome e posição original. Uma observação antiga não cura um monstro nem altera a vida do jogador. Conectar deixa de teleportar seu Link; **Reunir em Hateno** é manual.

**Experimental:** a correção recebeu validação estática e testes nativos/sintéticos e de rede. A renderização dos dois Links ainda precisa de confirmação em gameplay. PvP, colisão, coleta, objetos quebrados e IA/alvo mais próximo não estão integrados. O protocolo Gen1 completo continua bloqueado.

## Instalar

1. Feche os jogos, extraia o ZIP novo e abra **Instalar-Nativo.cmd**. Ele usa a instalação salva ou permite selecionar o Savage Wind 0.5.3-alpha.5. Guarda backup, testa a abertura do runtime e restaura os componentes anteriores em falha.
2. Abra a configuração do Savage Wind e **reconstrua a biblioteca do jogo** com o gerador atualizado. Não edite o arquivo `.id` para forçar compatibilidade.
3. Abra **Iniciar-Nativo.cmd** e o menu multiplayer. Use **Hospedar servidor** do pacote novo, sua interface/IP e a porta escolhida. **Os dois computadores precisam desta versão:** protocolo 3, incompatível com o servidor antigo e com o servidor Gen1 separado.
4. Conecte os clientes e escolha **Continuar**. Ambos devem estar na mesma área do mundo aberto. Use **Reunir em Hateno** se necessário; conectar sozinho não desloca o personagem.

O menu exige atualização da thread do jogo e só anuncia o ator após confirmar criação e componentes de modelo/animação. Modelo registrado não é prova de visibilidade na GPU. Falha de recursos, rotina parada ou tempo excedido geram erro explícito.

Em falha, mantenha o Cliente 1 aberto e execute **Diagnosticar.cmd**. O relatório inclui `game_ticks`, estado da criação, modelo/animação e monstros observados. `native_ready=false` refere-se à Gen1 completa, que continua incompleta.

## O que compartilha

- Vida dos monstros do mapa observados a até 140 m, no máximo 64 por leitura e 128 por sessão.
- Menor vida conhecida pelo servidor, incluindo zero. Não sincroniza ainda animação de morte, movimento ou autoridade da IA.
- Só um monstro carregado com o mesmo hash, nome e posição original recebe o valor. Endereços e IDs de processo nunca viajam pela rede.
- Saves, vida e inventário de cada jogador permanecem separados. O pacote não contém jogo, modelos, saves ou biblioteca recompilada.

Teste no mundo aberto. Missões, baús, itens no chão, árvores, clima, horário, Blood Moon e mudanças de mapa não são sincronizados. Reiniciar o servidor inicia outra sessão de estado compartilhado.

## Validação

- Callback real e armazenamento de HashId conferidos no módulo fornecido; treze assinaturas únicas.
- Testes C para controlador/dono, CPU, referências, criação/remoção e aplicação de vida somente na thread do jogo ao monstro correspondente.
- Testes com dois clientes para dano/vida zero convergindo, observações atrasadas sem cura e identidade inválida.
- Recursos derivados privadamente da instalação, preservando registros e subarquivos originais, sem controlador de física/dano do Player no proxy.

Pesquisa: [RESEARCH_SYNC.md](RESEARCH_SYNC.md). Runtime base [Savage Wind](https://github.com/Revivedx/SavageWindRecomp/tree/740b18025fe11ba03c117dbb56668db304b5043e), licença MIT. Nenhum código ou asset do Milk Bar foi incorporado.
