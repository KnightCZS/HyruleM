# Usar o protótipo 0.4.1

## Antes de conectar

O launcher usa o mesmo `config.json` das versões anteriores, em `%LOCALAPPDATA%\HyruleLauncher`. O sandbox original precisa existir e funcionar. O filtro de campanha, se habilitado, precisa estar preparado na tela principal. A configuração de backend deve ser Savage Wind, não Cemu/BotW-Native.

O ZIP de atualização pode ser extraído em uma nova pasta. Execute o novo launcher. Não é necessário sobrescrever seus arquivos do jogo. O servidor separado é opcional: o botão Hospedar já executa o mesmo servidor dentro do launcher.

## Um PC, dois jogos

Selecione Todas as interfaces e escolha uma porta TCP livre; clique Hospedar. Para os dois clientes locais, use o IP indicado no menu ou `127.0.0.1` com a mesma porta. Conecte os dois clientes. Não abra dois launchers: o mesmo menu gerencia as duas janelas de Savage Wind. Escolha Continuar em ambas e deixe Link parado até a mensagem de sincronização. O servidor atribui duas posições a quatro metros uma da outra na área inicial de Hateno.

Organizar 2 janelas restaura e posiciona os jogos lado a lado, com aproximadamente 16:9. Cada janela recebe uma projeção do avatar do outro cliente e seu nome. Ela acompanha a câmera real do jogo e não recebe mouse/teclado. O teclado controla a janela em foco; o protótipo não atribui um controle físico a cada processo. Para controlar dois jogadores simultaneamente, prefira dois PCs; um teste sequencial no mesmo teclado serve para verificar o movimento recebido.

O controle local usa duas portas efêmeras em `127.0.0.1`, diferentes da porta TCP do servidor. Não exponha as portas de controle do Savage Wind na rede.

## Dois PCs

Ambos precisam de jogo preparado e sandbox próprio. O anfitrião seleciona sua interface de rede e uma porta TCP, clica Hospedar e conecta seu cliente. Com Radmin, selecione o adaptador/IP Radmin. O convidado deve estar na mesma rede Radmin e usar o IP Radmin do anfitrião. O convidado informa o IPv4 do anfitrião e a porta escolhida; não hospeda outro servidor. Para uma LAN, use o IPv4 local. Para Internet, use uma VPN entre os PCs ou encaminhamento da porta TCP no roteador. O encaminhamento não é automático. Não existe autenticação: este servidor de protótipo é destinado a testes privados.

O servidor independente aceita `--listen`, `--spawn-x`, `--spawn-y`, `--spawn-z`; veja `HyruleM-Server.exe --help`. Encerre com Ctrl+C. O botão Hospedar do launcher usa o ponto padrão de Hateno.

## Nomes e ciclo da sessão

Nomes têm 1 a 24 caracteres; Mudar nome transmite a alteração para o outro cliente. A identidade de rede é gerada pelo servidor e não depende do nome. Desconectar rede deixa o jogo aberto; Conectar usa a mesma instância que ainda está rodando. Ao reconectar, ocorre nova tentativa de posicionamento no ponto inicial do servidor. Falha de teleporte mantém a rede conectada; use Reunir em Hateno para tentar novamente. Fechar o menu apenas o oculta; fechar o launcher encerra rede e camada visual, deixando os jogos abertos.

## Arquivos e isolamento

Dentro da pasta de dados escolhida na tela principal:

| Pasta | Uso |
| --- | --- |
| `hyrule-sandbox/save` | Sandbox original; lido como base, preservado |
| `hyrulem/client-1/hyrule-sandbox/save` | Save do cliente local 1 |
| `hyrulem/client-2/hyrule-sandbox/save` | Save do cliente local 2 |
| `hyrulem/client-N/savagewind.ini` | Configuração de vídeo/entrada individual |
| `hyrulem/client-N/extracted` | Vínculo para os mesmos recursos extraídos |

O save é copiado só ao preparar cada cliente pela primeira vez. Não há protocolo de inventário: cada instância continua com sua própria cópia local. Os dados originais e o sandbox existente não são regravados. O vínculo de pasta do Windows precisa ser suportado pelo volume (normalmente NTFS); não exige executar como administrador. Se a criação falhar, o launcher relata o erro em vez de duplicar o jogo inteiro.

Opções salvas em `launch-flags.txt` não são copiadas: podem alterar portas/saves ou iniciar re-execuções. Configurações normais de `savagewind.ini` são copiadas uma vez. O multiplayer ajusta apenas suas instâncias para janela, visão ampla e HDR desligado. Outras opções gráficas podem ser alteradas no menu de cada jogo. Monitore RAM/VRAM: dois jogos custam aproximadamente duas sessões normais.

## Limitações visuais

O avatar provisório não é o modelo de Link e não existe na lista de atores do jogo. Não tem colisão, oclusão pelas paredes, gestos/combate reais ou iluminação de BOTW. A direção é estimada pelo deslocamento; parado, mantém a última direção. A câmera é lida a aproximadamente 10 Hz; giros rápidos podem revelar atraso. O servidor envia estados a 20 Hz, com interpolação de 100 ms.

O enquadramento automático tenta ler, apenas para consulta, o bloco da câmera Switch 1.9.0 e valida seus campos. Se a leitura não corresponder ao formato esperado, usa FOV vertical efetivo manual, padrão 40°. Ajuste esse valor se o avatar deslizar lateralmente ao girar a câmera; o percentual de FOV do menu do Savage Wind não é o mesmo valor. Essa calibração ainda precisa de confirmação no jogo real. Não teste zoom de arco, câmera especial, diálogos ou cutscenes nesta fase. Em janelas fora de 16:9, o enquadramento é aproximado.

Não há identificação de instância de shrine, interior ou cena no protocolo: permaneçam no MainField, próximos de Hateno. Clima, hora e IA continuam locais e podem divergir. Essa etapa valida presença e movimento; o ator nativo é uma próxima integração, que exige alterações no runtime e no código gerado localmente.

## Diagnóstico

Logs em `%LOCALAPPDATA%\HyruleLauncher\logs`: `multiplayer-1.log`, `multiplayer-2.log`, `multiplayer-processo-1.log`, `multiplayer-processo-2.log`, `servidor.log`. Os logs normais do runtime ficam nos dados de cada instância.

- Conexão recusada: inicie o servidor e confira IP/porta/firewall.
- Servidor completo: existem duas conexões; desconecte uma ou reinicie o servidor de teste.
- Aguarda Continuar: carregue um save; antes de existir Link no mundo não há posição válida.
- Teleporte falhou: pare Link no mundo aberto, desconecte e reconecte. A rede permanece conectada. Clique Reunir em Hateno para repetir após ficar parado no mundo.
- Sem avatar: confirme ambos sincronizando, na mesma região, a menos de 300 m, e aponte a câmera para a outra posição. Use janela, SDR e 16:9. Camada visual some quando a câmera fica desatualizada ou a conexão cai.
- Janela minimizada: a camada se oculta. Restaurar o jogo a torna visível novamente.

## Atualização 0.4.1

O diagnóstico aberto pelo menu inclui os dois clientes e o servidor. Uma falha na preparação aparece antes de tentar conexão; uma falha ao abrir o processo preserva a mensagem original. O carregamento do jogo mantém a rede viva. A preparação do sandbox e o início do processo ocorrem fora da interface, com progresso agregado e botão de cancelamento. Consulte o [histórico](../CHANGELOG.md) e as [notas da versão](../releases/v0.4.1.md).
