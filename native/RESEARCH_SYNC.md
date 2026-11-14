# Atualização após as tarefas dos atores — issue #6

Os relatórios da prévia 6 confirmaram criação/visibilidade estática, mas nenhum movimento ou pedido AS: o resolver nativo retornou acesso negado durante toda a janela. A integração anterior dentro do job de dano do jogador podia atingir a proteção de acesso concorrente a outros atores. A correção atual substitui esse ponto pelo retorno da espera das tarefas dos atores, após o reset nativo da fase de acesso. A validação em execução privada com memória sintética reproduziu negação de um ator ativo no worker, espera de dois workers e resolução permitida após o reset. Não constitui teste gráfico.

ABI 4 exige recompilar a biblioteca com o gerador novo; protocolo 4 e recursos permanecem. O adaptador não desliga os checks nativos, não resolve por ponteiro bruto e não usa a thread do renderer. A captura local de AS mantém seu hook separado, somente de leitura.

## Histórico da integração anterior

# Comparação e correção da presença invisível

O relato do teste anterior confirmou conexão sem visibilidade do outro Link. A inspeção do módulo encontrou um erro suficiente para bloquear toda a criação: o parâmetro da rotina interceptada é o controlador de dano e o ator está em `this + 0x18`. O teste anterior usava diretamente um ator como parâmetro e não reproduzia a chamada do jogo. A correção e a regressão usam agora o controlador com seu dono.

| Referência primária | Mecânica estudada | Aplicação no HyruleM |
|---|---|---|
| [Milk Bar: Player](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/Player.cpp) e [LocalInstance](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/LocalInstance.h) | Ator próprio, configuração e ciclo de vida | A conexão é separada da confirmação de atualização, criação, modelo e animação |
| [Milk Bar: EnemyAccess](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/EnemyAccess.h) e [Enemy](https://github.com/MilkBarModding/MilkBarLauncher/blob/main/DLL/InjectDLL/Enemy.h) | Identidade de mapa e menor vida conhecida | Convergência de vida sem copiar a vida dos jogadores |
| [PrivCord: ghost Links](https://github.com/Blacknapoleon99/privcord-botw-coop/blob/main/docs/the-long-bug.md) e [arquitetura](https://github.com/Blacknapoleon99/privcord-botw-coop/blob/main/docs/how-it-works.md) | Mensagens TCP fragmentadas e distinção entre pose, criação e IA | Enquadramento por mensagem e testes de fragmentação; autoridade da IA permanece separada |
| [BreathM](https://github.com/msork/BreathM/blob/main/README.md) | O README Alpha 0.7 declara networking e presença; gameplay ainda não iniciado | Não fornece uma implementação pronta de atores ou mundo para portar |

Cemu usa Wii U/PowerPC, enquanto nosso runtime usa Switch 1.9.0/AArch64. Endereços e estruturas não são intercambiáveis. A implementação do HyruleM é original; nenhum código ou recurso do Milk Bar, cuja licença reserva todos os direitos, foi incorporado.

Vida compartilhada é uma primeira camada. IA por proximidade, coleta exclusiva, dano entre jogadores e objetos exigem callbacks adicionais. As regras Gen1 no servidor não tornam essas funcionalidades prontas no jogo.

## Retorno da issue #4

O relatório acessível informa `preview_abi: 1`, `hm_native_status` sem contadores de atualização e comando `hm_world_status` desconhecido. O SHA-256 do executável informado difere do binário publicado na prévia 2. Os outros dois relatórios tiveram conexão de controle recusada. Esses dados demonstram uso do adaptador antigo; não demonstram que a correção ABI 2 tenha executado.

A prévia 3 trata a instalação como parte do fluxo: atalho confere e instala no destino configurado, modo do EXE é selecionado pelo pacote, launcher exige identidade correspondente e a mensagem de falha sobrevive à limpeza da conexão. O instalador lê JSON UTF-8 explicitamente no Windows PowerShell. Não muda a implementação do ator nem anuncia confirmação de renderização.
