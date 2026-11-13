# Sandbox — 0.3.0

O sandbox usa um save próprio e ferramentas locais de exploração. A versão 0.3.0 acrescenta o filtro experimental de campanha descrito em [FILTRO_CAMPANHA.md](FILTRO_CAMPANHA.md).

## Continuar a sessão de Hateno

Feche o jogo e o launcher anterior, extraia a atualização e abra o novo executável. Os caminhos e o save existente são reaproveitados. Use **Jogar sandbox** e depois **Continuar** na recomp. Não use Novo + backup para simplesmente atualizar o programa ou o filtro.

O perfil padrão é **Campanha bloqueada · secundárias e santuários**. Na primeira execução ele prepara os mods localmente. **Compatibilidade · manter campanha original** permite retomar o mesmo save sem o filtro.

## Criar outro sandbox

É necessário Savage Wind com BOTW Switch 1.9.0 preparado e um save normal que já funcione nessa instalação. Saves Wii U/Cemu devem ser importados pelo Savage Wind antes.

Escolha a região inicial e use Jogar sandbox quando ainda não existir um sandbox, ou Novo + backup para criar outro a partir da base. A origem automática é o slot normal mais recente do perfil de campanha ativo. Também é possível escolher `game_data.sav`; o `caption.sav` correspondente precisa estar na mesma pasta.

A criação copia a base, prepara o paraglider e muda a posição salva para a região escolhida. Não concede runas nem ativa torres. As outras habilidades, itens e estatísticas vêm da base. O save original é apenas lido.

Jogar sandbox retoma o progresso existente. Trocar a região na lista não reinicia o save nem reposiciona Link a cada carregamento. Novo + backup preserva o sandbox anterior em uma pasta com sufixo de data antes de publicar o novo.

## Ferramentas de sessão

Com o mundo carregado e o jogo renderizando, escolha uma ferramenta e clique em Aplicar:

- consultar posição;
- definir 12:00 ou 22:00;
- congelar ou liberar o horário;
- céu limpo, chuva ou clima natural;
- definir a carteira em 1.000 rupias;
- ativar ou desativar voo;
- solicitar espada de viajante, escudo de madeira e dez maçãs.

O kit depende de espaço no inventário; confira os itens e os logs. Uma resposta aceita não prova que todas as armas entraram.

Teleportar usa a região selecionada e acompanha o carregamento do mapa. Faça isso com Link parado, fora de cutscenes, montarias e carregamentos. Não repita um teleporte enquanto ele ainda estiver em andamento. Se o jogo pausar ao perder foco, mantenha-o em janela e permita a renderização em segundo plano na recomp.

Horário, clima e voo são controles da sessão. Inventário, dinheiro, posição e progressão podem persistir quando o próprio jogo salvar.

## Arquivos

Em `<pasta de dados>/hyrule-sandbox` ficam o save e o relatório de sua criação. O filtro fica separado, em `hyrule-sandbox-filter`, e sua política em `hyrule-sandbox-policy.json`. Atualizar o filtro não recria o save.

O campo histórico `CampaignRemoved` de relatórios 0.2 não descreve a camada de mods atual. Consulte `hyrule-sandbox-filter/filter.json` para o filtro da versão 0.3.

Jogar campanha usa o perfil normal do Savage Wind. O launcher não altera `save_profile.txt` nem define variáveis permanentes do Windows.

O usuário confirmou o início em Hateno na versão 0.2, sem torres ou runas. O novo filtro 0.3 foi validado por testes de formato e integração; sua execução no BOTW real ainda precisa ser confirmada. Não há multiplayer ou servidor RP nesta versão.
