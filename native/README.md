# HyruleM 0.5.0-preview.3

Mantém a correção da rotina que ignorava os pedidos de criação do Link remoto: o callback recebe um controlador de dano, cujo dono fica em `this + 0x18`. A implementação inicial comparava o controlador diretamente com o jogador e saía antes da fila. A relação foi conferida no módulo 1.9.0 e os testes reproduzem agora a chamada real.

Inclui a primeira integração de mundo: convergência de vida dos monstros próximos, identificados por hash do mapa, nome e posição original. Uma observação antiga não cura um monstro nem altera a vida do jogador. Conectar deixa de teleportar seu Link; **Reunir em Hateno** é manual.

**Experimental:** a correção recebeu validação estática e testes nativos/sintéticos e de rede. A renderização dos dois Links ainda precisa de confirmação em gameplay. PvP, colisão, coleta, objetos quebrados e IA/alvo mais próximo não estão integrados. O protocolo Gen1 completo continua bloqueado.

## Instalar

Os relatórios da [issue #4](https://github.com/KnightCZS/HyruleM/issues/4) mostram ABI 1 e um executável diferente do pacote da prévia 2. O teste usou o runtime antigo. Esta atualização confere a instalação selecionada e exige ABI 2 antes de preparar o multiplayer.

1. Feche os jogos **e o launcher antigo**. Extraia o ZIP novo inteiro em uma pasta nova.
2. Execute **Iniciar-Nativo.cmd**. Ele confere todos os componentes do pacote na instalação salva no launcher e atualiza os que não correspondem, guardando backup. Em falha, restaura os componentes anteriores e não abre o launcher. A confirmação mostra o executável instalado e ABI 2.
3. Se estava usando ABI 1, abra a configuração do Savage Wind e **reconstrua a biblioteca do jogo** com o gerador atualizado. Não edite o arquivo `.id` para forçar compatibilidade.
4. Volte ao launcher deste pacote, use **Hospedar servidor** e conecte os clientes. **Os dois computadores precisam da atualização:** protocolo 3, incompatível com o servidor antigo e com o servidor Gen1 separado.
5. Escolha **Continuar** nos jogos. Ambos devem estar na mesma área do mundo aberto. **Reunir em Hateno** é manual.

Abrir `HyruleLauncher.exe` diretamente dentro deste pacote também ativa o modo nativo pelo manifesto `hyrulem-preview.json`; o modo não depende mais de uma variável definida só pelo atalho. Se o runtime selecionado não corresponde ao pacote, o menu informa o caminho e exige atualização. Extraia todos os arquivos juntos; copiar apenas o EXE não constitui uma atualização completa.

O menu exige atualização da thread do jogo e só anuncia o ator após confirmar criação e componentes de modelo/animação. Modelo registrado não é prova de visibilidade na GPU. A causa de uma falha nativa permanece após desconectar.

Em falha, mantenha os dois clientes abertos e execute **Diagnosticar.cmd**. Ele gera um relatório por cliente com `native-preview-diag.3`. O campo `preview` distingue runtime antigo, biblioteca sem atualização, callback parado e estado do ator. Dois relatórios antigos tinham conexão de controle recusada: esses relatórios não observavam um jogo acessível naquele momento. `native_ready=false` continua referindo-se à Gen1 completa, não à compatibilidade desta prévia.

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
