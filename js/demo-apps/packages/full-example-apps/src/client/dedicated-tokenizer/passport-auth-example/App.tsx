import { CssBaseline, makeStyles } from '@material-ui/core';
import { createStyles, Theme } from '@material-ui/core/styles';
import React from 'react';
import { BrowserRouter, Route, Switch } from 'react-router-dom'; // Pages

import { Documents } from '../../shared-components/Documents';
import { Header } from '../../shared-components/Header';
import { Home } from '../../shared-components/Home';
import { Login } from '../../shared-components/Login';
import { SideMenu } from '../../shared-components/SideMenu';
import { Signup } from '../../shared-components/Signup';
import { Usage } from '../../shared-components/Usage';
import { User } from '../../shared-components/User';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    root: {
      display: 'flex',
    },
    main: {
      flexGrow: 1,
      padding: theme.spacing(3),
    },
    toolbar: theme.mixins.toolbar,
  }),
);

export const App = () => {
  const classes = useStyles({});

  return (
    <BrowserRouter>
      <div className={classes.root}>
        <CssBaseline />
        <Header />
        <SideMenu />
        <main className={classes.main}>
          <div className={classes.toolbar} />
          <Switch>
            <Route exact path='/' component={Home} />
            <Route exact path='/login' component={Login} />
            <Route exact path='/usage' component={Usage} />
            <Route exact path='/signup' component={Signup} />
            <Route exact path='/user' component={User} />
            <Route exact path='/documents' component={Documents} />
          </Switch>
        </main>
      </div>
    </BrowserRouter>
  );
};
